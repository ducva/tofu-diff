package plan

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/encoding/protowire"
)

func init() {
	// Register a handler for msgpack extension type 0, which represents
	// cty "unknown" values (values not known until after apply).
	msgpack.RegisterExt(0, (*msgpackUnknown)(nil))
	// Register a handler for msgpack extension type 12, which represents
	// cty "refined unknown" values.
	msgpack.RegisterExt(12, (*msgpackUnknown)(nil))
}

// msgpackUnknown is a sentinel type used to detect cty unknown extension markers
// in msgpack-encoded plan values.
type msgpackUnknown struct{}

func (u *msgpackUnknown) UnmarshalMsgpack(data []byte) error { return nil }
func (u msgpackUnknown) MarshalMsgpack() ([]byte, error)     { return nil, nil }

type ActionType string

const (
	ActionCreate  ActionType = "create"
	ActionUpdate  ActionType = "update"
	ActionDelete  ActionType = "delete"
	ActionReplace ActionType = "replace"
	ActionNoOp    ActionType = "no-op"
)

type Change struct {
	Actions         []string                   `json:"actions"`
	Before          map[string]json.RawMessage `json:"before"`
	After           map[string]json.RawMessage `json:"after"`
	AfterUnknown    map[string]bool            `json:"after_unknown"`
	BeforeSensitive map[string]bool            `json:"before_sensitive"`
	AfterSensitive  map[string]bool            `json:"after_sensitive"`
}

func (c *Change) NormalizedAction() ActionType {
	if len(c.Actions) == 0 {
		return ActionNoOp
	}
	if len(c.Actions) == 2 {
		return ActionReplace
	}
	switch c.Actions[0] {
	case "create":
		return ActionCreate
	case "delete":
		return ActionDelete
	case "update":
		return ActionUpdate
	case "no-op":
		return ActionNoOp
	default:
		return ActionNoOp
	}
}

type ResourceChange struct {
	Address       string `json:"address"`
	ModuleAddress string `json:"module_address"`
	Mode          string `json:"mode"`
	Type          string `json:"type"`
	Name          string `json:"name"`
	Change        Change `json:"change"`
}

type PlanFile struct {
	FormatVersion   string           `json:"format_version"`
	ResourceChanges []ResourceChange `json:"resource_changes"`
}

type AttributeDiff struct {
	Key             string
	BeforeDisplay   string
	AfterDisplay    string
	BeforeRaw       json.RawMessage
	AfterRaw        json.RawMessage
	BeforeSensitive bool
	AfterSensitive  bool
	IsUnknownAfter  bool
	IsSensitive     bool
}

// DiffAttributes computes the attribute-level diff for a Change.
func DiffAttributes(c Change) []AttributeDiff {
	keySet := make(map[string]struct{})
	for k := range c.Before {
		keySet[k] = struct{}{}
	}
	for k := range c.After {
		keySet[k] = struct{}{}
	}
	for k := range c.AfterUnknown {
		keySet[k] = struct{}{}
	}

	var diffs []AttributeDiff
	for key := range keySet {
		if c.AfterUnknown[key] {
			diffs = append(diffs, AttributeDiff{
				Key:             key,
				BeforeDisplay:   FormatValue(c.Before[key], c.BeforeSensitive[key]),
				AfterDisplay:    "(known after apply)",
				BeforeRaw:       c.Before[key],
				BeforeSensitive: c.BeforeSensitive[key],
				IsUnknownAfter:  true,
			})
			continue
		}
		if bytes.Equal(c.Before[key], c.After[key]) {
			continue
		}
		diffs = append(diffs, AttributeDiff{
			Key:             key,
			BeforeDisplay:   FormatValue(c.Before[key], c.BeforeSensitive[key]),
			AfterDisplay:    FormatValue(c.After[key], c.AfterSensitive[key]),
			BeforeRaw:       c.Before[key],
			AfterRaw:        c.After[key],
			BeforeSensitive: c.BeforeSensitive[key],
			AfterSensitive:  c.AfterSensitive[key],
		})
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Key < diffs[j].Key
	})
	return diffs
}

// FormatValue formats a raw JSON attribute value for display.
func FormatValue(raw json.RawMessage, sensitive bool) string {
	if sensitive {
		return "(sensitive)"
	}
	if raw == nil || string(raw) == "null" {
		return "null"
	}
	if len(raw) > 0 && raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return Truncate(s)
		}
	}
	return Truncate(string(raw))
}

// Truncate clips a string to 120 characters with a "..." suffix.
func Truncate(s string) string {
	const max = 120
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// zipMagic is the magic number for ZIP files.
var zipMagic = []byte("PK\x03\x04")

// isBinaryPlan returns true if data looks like a binary plan (ZIP-wrapped protobuf).
func isBinaryPlan(data []byte) bool {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	return bytes.HasPrefix(trimmed, zipMagic)
}

// ---------------------------------------------------------------------------
// binary plan (ZIP) parsing via tfplan protobuf
// ---------------------------------------------------------------------------

// readZipEntry reads the full contents of a zip file entry.
func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// parseAddress splits a full resource address into type, name, and module parts.
// e.g. "module.foo.snowflake_table.this[\"key\"]" → type="snowflake_table", name="this"
func parseAddress(addr string) (typ, name, moduleAddr string) {
	bare := addr
	if idx := strings.Index(addr, "["); idx >= 0 {
		bare = addr[:idx]
	}
	lastDot := strings.LastIndex(bare, ".")
	if lastDot < 0 {
		return "", "", ""
	}
	name = bare[lastDot+1:]
	rest := bare[:lastDot]
	secondLast := strings.LastIndex(rest, ".")
	if secondLast < 0 {
		typ = rest
		return typ, name, ""
	}
	typ = rest[secondLast+1:]
	moduleAddr = rest[:secondLast]
	return typ, name, moduleAddr
}

// decodeProtoPathSet decodes a PathSet message (repeated Path) into a set of
// top-level attribute names.
func decodeProtoPathSet(data []byte) map[string]bool {
	result := make(map[string]bool)
	offset := 0
	for offset < len(data) {
		fn, wt, n := protowire.ConsumeTag(data[offset:])
		if n < 0 {
			break
		}
		offset += n
		if fn == 1 && wt == protowire.BytesType {
			pathData, n := protowire.ConsumeBytes(data[offset:])
			if n < 0 {
				break
			}
			offset += n
			attrName := decodeProtoPath(pathData)
			if attrName != "" {
				result[attrName] = true
			}
		} else {
			n := protowire.ConsumeFieldValue(fn, wt, data[offset:])
			if n < 0 {
				break
			}
			offset += n
		}
	}
	return result
}

// decodeProtoPath decodes a Path message and returns the first attribute name.
func decodeProtoPath(data []byte) string {
	offset := 0
	for offset < len(data) {
		fn, wt, n := protowire.ConsumeTag(data[offset:])
		if n < 0 {
			break
		}
		offset += n
		if fn == 1 && wt == protowire.BytesType {
			stepData, n := protowire.ConsumeBytes(data[offset:])
			if n < 0 {
				break
			}
			offset += n
			attr := decodeProtoStep(stepData)
			if attr != "" {
				return attr
			}
		} else {
			n := protowire.ConsumeFieldValue(fn, wt, data[offset:])
			if n < 0 {
				break
			}
			offset += n
		}
	}
	return ""
}

// decodeProtoStep decodes a Step message and returns the attribute_name or element_key.
func decodeProtoStep(data []byte) string {
	offset := 0
	for offset < len(data) {
		fn, wt, n := protowire.ConsumeTag(data[offset:])
		if n < 0 {
			break
		}
		offset += n
		if (fn == 1 || fn == 2) && wt == protowire.BytesType {
			val, n := protowire.ConsumeBytes(data[offset:])
			if n < 0 {
				break
			}
			offset += n
			return string(val)
		}
		n = protowire.ConsumeFieldValue(fn, wt, data[offset:])
		if n < 0 {
			break
		}
		offset += n
	}
	return ""
}

// decodeDynamicValue decodes a DynamicValue message and returns the msgpack bytes.
func decodeDynamicValue(data []byte) []byte {
	offset := 0
	for offset < len(data) {
		fn, wt, n := protowire.ConsumeTag(data[offset:])
		if n < 0 {
			break
		}
		offset += n
		if fn == 1 && wt == protowire.BytesType {
			mp, n := protowire.ConsumeBytes(data[offset:])
			if n < 0 {
				break
			}
			return mp
		}
		n = protowire.ConsumeFieldValue(fn, wt, data[offset:])
		if n < 0 {
			break
		}
		offset += n
	}
	return nil
}

// protoActionsToActions converts a proto action to the Actions slice.
func protoActionsToActions(action uint64) []string {
	switch action {
	case 6: // DELETE_THEN_CREATE
		return []string{"delete", "create"}
	case 7: // CREATE_THEN_DELETE
		return []string{"create", "delete"}
	default:
		return []string{protoActionName(action)}
	}
}

func protoActionName(a uint64) string {
	switch a {
	case 0:
		return "no-op"
	case 1:
		return "create"
	case 2:
		return "read"
	case 3:
		return "update"
	case 5:
		return "delete"
	case 6:
		return "delete_then_create"
	case 7:
		return "create_then_delete"
	default:
		return "no-op"
	}
}

// decodeMsgpackAttrs decodes msgpack bytes into a map of JSON attribute values
// and the set of keys that contain cty unknown markers.
func decodeMsgpackAttrs(mpData []byte) (map[string]json.RawMessage, map[string]bool, error) {
	if len(mpData) == 0 {
		return nil, nil, nil
	}

	var v interface{}
	if err := msgpack.Unmarshal(mpData, &v); err != nil {
		return nil, nil, fmt.Errorf("msgpack decode: %w", err)
	}

	mv, ok := v.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("expected msgpack map, got %T", v)
	}

	// Collect keys that contain unknown markers
	unknownKeys := collectUnknownKeys(mv)

	// Replace unknown markers with nil and marshal each value to JSON
	result := make(map[string]json.RawMessage)
	for key, val := range mv {
		replaced := replaceUnknowns(val)
		b, err := json.Marshal(replaced)
		if err != nil {
			return nil, nil, fmt.Errorf("json marshal key %q: %w", key, err)
		}
		result[key] = json.RawMessage(b)
	}

	return result, unknownKeys, nil
}

// collectUnknownKeys returns the set of top-level keys that contain a cty
// unknown marker anywhere in their nested value structure.
func collectUnknownKeys(mv map[string]interface{}) map[string]bool {
	result := make(map[string]bool)
	for key, val := range mv {
		if containsUnknown(val) {
			result[key] = true
		}
	}
	return result
}

// containsUnknown recursively checks whether a value contains a msgpackUnknown.
func containsUnknown(v interface{}) bool {
	switch v.(type) {
	case *msgpackUnknown, msgpackUnknown:
		return true
	case []interface{}:
		for _, elem := range v.([]interface{}) {
			if containsUnknown(elem) {
				return true
			}
		}
	case map[string]interface{}:
		for _, elem := range v.(map[string]interface{}) {
			if containsUnknown(elem) {
				return true
			}
		}
	}
	return false
}

// replaceUnknowns recursively replaces msgpackUnknown sentinels with nil.
func replaceUnknowns(v interface{}) interface{} {
	switch v.(type) {
	case *msgpackUnknown, msgpackUnknown:
		return nil
	case []interface{}:
		val := v.([]interface{})
		out := make([]interface{}, len(val))
		for i, elem := range val {
			out[i] = replaceUnknowns(elem)
		}
		return out
	case map[string]interface{}:
		val := v.(map[string]interface{})
		out := make(map[string]interface{}, len(val))
		for k, elem := range val {
			out[k] = replaceUnknowns(elem)
		}
		return out
	default:
		return v
	}
}

// loadBinaryBytes parses a binary plan from raw bytes (ZIP-wrapped protobuf).
func loadBinaryBytes(data []byte) (*PlanFile, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("cannot open binary plan as zip: %w", err)
	}

	entries := make(map[string]*zip.File)
	for _, f := range zr.File {
		entries[f.Name] = f
	}

	tfplanEntry, ok := entries["tfplan"]
	if !ok {
		return nil, fmt.Errorf("binary plan zip is missing tfplan")
	}

	tfplanData, err := readZipEntry(tfplanEntry)
	if err != nil {
		return nil, fmt.Errorf("cannot read tfplan: %w", err)
	}

	var changes []ResourceChange

	offset := 0
	for offset < len(tfplanData) {
		fn, wt, n := protowire.ConsumeTag(tfplanData[offset:])
		if n < 0 {
			break
		}
		offset += n

		if fn == 3 && wt == protowire.BytesType {
			rcData, n := protowire.ConsumeBytes(tfplanData[offset:])
			if n < 0 {
				break
			}
			offset += n

			rc, err := decodeResourceChange(rcData)
			if err != nil {
				return nil, fmt.Errorf("decode resource change: %w", err)
			}
			changes = append(changes, rc)
		} else {
			n = protowire.ConsumeFieldValue(fn, wt, tfplanData[offset:])
			if n < 0 {
				break
			}
			offset += n
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Address < changes[j].Address
	})

	return &PlanFile{
		FormatVersion:   "1.0",
		ResourceChanges: changes,
	}, nil
}

// decodeResourceChange parses a ResourceInstanceChange protobuf message.
func decodeResourceChange(data []byte) (ResourceChange, error) {
	var (
		addr       string
		changeData []byte
	)

	offset := 0
	for offset < len(data) {
		fn, wt, n := protowire.ConsumeTag(data[offset:])
		if n < 0 {
			break
		}
		offset += n

		switch {
		case fn == 13 && wt == protowire.BytesType: // addr
			val, n := protowire.ConsumeBytes(data[offset:])
			if n < 0 {
				break
			}
			offset += n
			addr = string(val)

		case fn == 9 && wt == protowire.BytesType: // change
			val, n := protowire.ConsumeBytes(data[offset:])
			if n < 0 {
				break
			}
			offset += n
			changeData = val

		default:
			n = protowire.ConsumeFieldValue(fn, wt, data[offset:])
			if n < 0 {
				break
			}
			offset += n
		}
	}

	if addr == "" {
		return ResourceChange{}, fmt.Errorf("resource change missing address")
	}

	typ, name, moduleAddr := parseAddress(addr)

	if changeData == nil {
		return ResourceChange{
			Address:       addr,
			ModuleAddress: moduleAddr,
			Mode:          "managed",
			Type:          typ,
			Name:          name,
			Change: Change{
				Actions: []string{"no-op"},
			},
		}, nil
	}

	action, beforeMp, afterMp, beforeSens, afterSens, err := decodeChange(changeData)
	if err != nil {
		return ResourceChange{}, err
	}

	var before, after map[string]json.RawMessage
	var afterUnknown map[string]bool

	if len(beforeMp) > 0 {
		before, _, err = decodeMsgpackAttrs(beforeMp)
		if err != nil {
			return ResourceChange{}, fmt.Errorf("decode before: %w", err)
		}
	}

	if len(afterMp) > 0 {
		after, afterUnknown, err = decodeMsgpackAttrs(afterMp)
		if err != nil {
			return ResourceChange{}, fmt.Errorf("decode after: %w", err)
		}
	}

	return ResourceChange{
		Address:       addr,
		ModuleAddress: moduleAddr,
		Mode:          "managed",
		Type:          typ,
		Name:          name,
		Change: Change{
			Actions:         protoActionsToActions(action),
			Before:          before,
			After:           after,
			AfterUnknown:    afterUnknown,
			BeforeSensitive: beforeSens,
			AfterSensitive:  afterSens,
		},
	}, nil
}

// decodeChange parses a Change protobuf message.
func decodeChange(data []byte) (action uint64, beforeMp, afterMp []byte, beforeSens, afterSens map[string]bool, err error) {
	var values [][]byte

	offset := 0
	for offset < len(data) {
		fn, wt, n := protowire.ConsumeTag(data[offset:])
		if n < 0 {
			break
		}
		offset += n

		switch {
		case fn == 1 && wt == protowire.VarintType: // action
			v, n := protowire.ConsumeVarint(data[offset:])
			if n < 0 {
				break
			}
			offset += n
			action = v

		case fn == 2 && wt == protowire.BytesType: // values (repeated DynamicValue)
			dv, n := protowire.ConsumeBytes(data[offset:])
			if n < 0 {
				break
			}
			offset += n
			mp := decodeDynamicValue(dv)
			values = append(values, mp)

		case fn == 3 && wt == protowire.BytesType: // before_sensitive_paths
			psData, n := protowire.ConsumeBytes(data[offset:])
			if n < 0 {
				break
			}
			offset += n
			beforeSens = decodeProtoPathSet(psData)

		case fn == 4 && wt == protowire.BytesType: // after_sensitive_paths
			psData, n := protowire.ConsumeBytes(data[offset:])
			if n < 0 {
				break
			}
			offset += n
			afterSens = decodeProtoPathSet(psData)

		default:
			n = protowire.ConsumeFieldValue(fn, wt, data[offset:])
			if n < 0 {
				break
			}
			offset += n
		}
	}

	if beforeSens == nil {
		beforeSens = make(map[string]bool)
	}
	if afterSens == nil {
		afterSens = make(map[string]bool)
	}

	// Determine before/after from action and values
	switch action {
	case 0: // NOOP — values[0] is the unchanged value
		if len(values) > 0 {
			beforeMp = values[0]
			afterMp = values[0]
		}
	case 1: // CREATE — values[0] is the after (planned) value
		if len(values) > 0 {
			afterMp = values[0]
		}
	case 3: // UPDATE — values[0] is before, values[1] is after
		if len(values) > 0 {
			beforeMp = values[0]
		}
		if len(values) > 1 {
			afterMp = values[1]
		}
	case 5: // DELETE — values[0] is the before (existing) value
		if len(values) > 0 {
			beforeMp = values[0]
		}
	case 6: // DELETE_THEN_CREATE — values[0] before, values[1] after
		if len(values) > 0 {
			beforeMp = values[0]
		}
		if len(values) > 1 {
			afterMp = values[1]
		}
	case 7: // CREATE_THEN_DELETE — values[0] after, values[1] before
		if len(values) > 0 {
			afterMp = values[0]
		}
		if len(values) > 1 {
			beforeMp = values[1]
		}
	}

	return action, beforeMp, afterMp, beforeSens, afterSens, nil
}

// Load reads a plan file (JSON or binary) from the given path and returns
// the parsed PlanFile.
func Load(path string) (*PlanFile, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()
	return LoadReader(f, path)
}

// LoadReader reads a plan (JSON or binary) from an io.Reader.
// The name parameter is used in error messages to identify the source
// (e.g., a file path or "stdin").
func LoadReader(r io.Reader, name string) (*PlanFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", name, err)
	}

	if isBinaryPlan(data) {
		return loadBinaryBytes(data)
	}

	var pf PlanFile
	if err := json.Unmarshal(data, &pf); err != nil {
		// If the data doesn't even look like JSON, the user likely piped
		// `tofu plan` directly (which outputs human-readable text).
		if !bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte("{")) {
			return nil, fmt.Errorf(
				"input from %s is not a valid JSON or binary tofu plan.\n\n"+
					"Hint: Did you pipe 'tofu plan' directly? That outputs human-readable text.\n"+
					"To get a JSON plan, use one of these patterns:\n"+
					"  tofu plan -out=tfplan && tofu show -json tfplan | tofu-diff\n"+
					"  tofu plan -out=tfplan && tofu show -json tfplan > plan.json && tofu-diff plan.json\n"+
					"  tofu-diff tfplan  (reads the binary plan file directly)",
				name,
			)
		}
		return nil, fmt.Errorf("not a valid tofu JSON plan file from %s: %w", name, err)
	}

	if pf.FormatVersion != "1.0" {
		fmt.Fprintf(os.Stderr, "warning: unrecognized plan format_version %q; output may be incorrect\n", pf.FormatVersion)
	}

	return &pf, nil
}
