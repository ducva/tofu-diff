package render

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/ducva/tofu-diff/plan"
)

type PlanRenderer struct {
	out      io.Writer
	diffOnly bool
}

func New(out io.Writer) *PlanRenderer {
	return &PlanRenderer{out: out, diffOnly: true}
}

func NewWithDiffOnly(out io.Writer, diffOnly bool) *PlanRenderer {
	return &PlanRenderer{out: out, diffOnly: diffOnly}
}

func (r *PlanRenderer) SetDiffOnly(v bool) {
	r.diffOnly = v
}

func (r *PlanRenderer) Render(pf plan.PlanFile) error {
	bw := bufio.NewWriter(r.out)
	defer bw.Flush()

	printed := 0
	for _, rc := range pf.ResourceChanges {
		if rc.Change.NormalizedAction() == plan.ActionNoOp {
			continue
		}
		r.renderResource(bw, rc)
		printed++
	}

	if printed == 0 {
		fmt.Fprintf(bw, "No changes. Infrastructure is up-to-date.\n")
	}

	return nil
}

func (r *PlanRenderer) renderResource(w io.Writer, rc plan.ResourceChange) {
	action := rc.Change.NormalizedAction()
	fmt.Fprintf(w, "%s %s\n", r.actionLabel(action), rc.Address)

	switch action {
	case plan.ActionCreate:
		for _, key := range sortedKeys(rc.Change.After) {
			if rc.Change.AfterUnknown[key] {
				fmt.Fprintf(w, "  + %s = (known after apply)\n", key)
			} else {
				val := plan.FormatValue(rc.Change.After[key], rc.Change.AfterSensitive[key])
				fmt.Fprintf(w, "  + %s = %s\n", key, val)
			}
		}
	case plan.ActionDelete:
		for _, key := range sortedKeys(rc.Change.Before) {
			val := plan.FormatValue(rc.Change.Before[key], rc.Change.BeforeSensitive[key])
			fmt.Fprintf(w, "  - %s = %s\n", key, val)
		}
	case plan.ActionUpdate, plan.ActionReplace:
		if r.diffOnly {
			for _, d := range plan.DiffAttributes(rc.Change) {
				fmt.Fprintf(w, "  ~ %s: %s -> %s\n", d.Key, d.BeforeDisplay, d.AfterDisplay)
			}
		} else {
			// Full context: show all attributes including unchanged ones.
			keys := sortedKeysUnion(rc.Change.Before, rc.Change.After, rc.Change.AfterUnknown)
			for _, key := range keys {
				if rc.Change.AfterUnknown[key] {
					beforeVal := plan.FormatValue(rc.Change.Before[key], rc.Change.BeforeSensitive[key])
					fmt.Fprintf(w, "  ~ %s: %s -> (known after apply)\n", key, beforeVal)
					continue
				}
				beforeRaw := rc.Change.Before[key]
				afterRaw := rc.Change.After[key]
				if bytesEqual(beforeRaw, afterRaw) {
					val := plan.FormatValue(beforeRaw, rc.Change.BeforeSensitive[key] || rc.Change.AfterSensitive[key])
					fmt.Fprintf(w, "    %s = %s\n", key, val)
					continue
				}
				if beforeRaw == nil && afterRaw != nil {
					val := plan.FormatValue(afterRaw, rc.Change.AfterSensitive[key])
					fmt.Fprintf(w, "  + %s = %s\n", key, val)
					continue
				}
				if afterRaw == nil && beforeRaw != nil {
					val := plan.FormatValue(beforeRaw, rc.Change.BeforeSensitive[key])
					fmt.Fprintf(w, "  - %s = %s\n", key, val)
					continue
				}
				beforeVal := plan.FormatValue(beforeRaw, rc.Change.BeforeSensitive[key])
				afterVal := plan.FormatValue(afterRaw, rc.Change.AfterSensitive[key])
				fmt.Fprintf(w, "  ~ %s: %s -> %s\n", key, beforeVal, afterVal)
			}
		}
	}

	fmt.Fprintln(w)
}

func (r *PlanRenderer) actionLabel(action plan.ActionType) string {
	switch action {
	case plan.ActionCreate:
		return "[+] create:"
	case plan.ActionDelete:
		return "[-] destroy:"
	case plan.ActionUpdate:
		return "[~] update:"
	case plan.ActionReplace:
		return "[±] replace:"
	default:
		return "[?] unknown:"
	}
}

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysUnion(before, after map[string]json.RawMessage, afterUnknown map[string]bool) []string {
	keySet := make(map[string]struct{})
	for k := range before {
		keySet[k] = struct{}{}
	}
	for k := range after {
		keySet[k] = struct{}{}
	}
	for k := range afterUnknown {
		keySet[k] = struct{}{}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func bytesEqual(a, b json.RawMessage) bool {
	return bytes.Equal(a, b)
}
