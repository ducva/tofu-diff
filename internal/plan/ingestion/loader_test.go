package ingestion

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ducva/tofu-diff/internal/plan/domain"
	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestLoadReaderJSON(t *testing.T) {
	input := `{
		"format_version":"1.0",
		"resource_changes":[{
			"address":"test.example",
			"mode":"managed",
			"type":"test",
			"name":"example",
			"change":{"actions":["create"],"before":null,"after":{"name":"value"}}
		}]
	}`

	plan, err := LoadReader(strings.NewReader(input), "fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ResourceChanges) != 1 {
		t.Fatalf("got %d resource changes, want 1", len(plan.ResourceChanges))
	}
	if got := plan.ResourceChanges[0].Change.NormalizedAction(); got != domain.ActionCreate {
		t.Fatalf("got action %q, want create", got)
	}
}

func TestLoadReaderJSONWithComplexSensitivityAndUnknown(t *testing.T) {
	input := `{
		"format_version":"1.0",
		"resource_changes":[{
			"address":"test.example",
			"mode":"managed",
			"type":"test",
			"name":"example",
			"change":{
				"actions":["update"],
				"before":{"sens":"old", "normal":"1"},
				"after":{"sens":"new", "normal":"2", "computed":"3"},
				"after_unknown":{"computed":true, "normal":false},
				"before_sensitive":{"sens":[{"nested":true}], "normal":false},
				"after_sensitive":{"sens":true, "normal":false}
			}
		}]
	}`

	plan, err := LoadReader(strings.NewReader(input), "fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ResourceChanges) != 1 {
		t.Fatalf("got %d resource changes, want 1", len(plan.ResourceChanges))
	}
	rc := plan.ResourceChanges[0]
	if !rc.Change.BeforeSensitive["sens"] {
		t.Error("expected before_sensitive['sens'] to be true")
	}
	if rc.Change.BeforeSensitive["normal"] {
		t.Error("expected before_sensitive['normal'] to be false")
	}
	if !rc.Change.AfterSensitive["sens"] {
		t.Error("expected after_sensitive['sens'] to be true")
	}
	if !rc.Change.AfterUnknown["computed"] {
		t.Error("expected after_unknown['computed'] to be true")
	}
}

func TestLoadReaderRejectsUnsupportedAction(t *testing.T) {
	input := `{"format_version":"1.0","resource_changes":[{"address":"test.example","change":{"actions":["move"]}}]}`
	if _, err := LoadReader(strings.NewReader(input), "fixture.json"); err == nil {
		t.Fatal("LoadReader accepted unsupported action")
	}
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		addr       string
		wantType   string
		wantName   string
		wantModule string
	}{
		{
			addr:       "aws_s3_bucket.my_bucket",
			wantType:   "aws_s3_bucket",
			wantName:   "my_bucket",
			wantModule: "",
		},
		{
			addr:       "aws_s3_bucket.my_bucket[0]",
			wantType:   "aws_s3_bucket",
			wantName:   "my_bucket",
			wantModule: "",
		},
		{
			addr:       "module.vpc.aws_subnet.public",
			wantType:   "aws_subnet",
			wantName:   "public",
			wantModule: "module.vpc",
		},
		{
			addr:       "module.vpc.aws_subnet.public[\"key\"]",
			wantType:   "aws_subnet",
			wantName:   "public",
			wantModule: "module.vpc",
		},
		{
			addr:       "module.vpc[0].aws_subnet.public",
			wantType:   "aws_subnet",
			wantName:   "public",
			wantModule: "module.vpc[0]",
		},
		{
			addr:       "module.vpc[0].module.sub.aws_subnet.public[1]",
			wantType:   "aws_subnet",
			wantName:   "public",
			wantModule: "module.vpc[0].module.sub",
		},
	}

	for _, tt := range tests {
		typ, name, mod := parseAddress(tt.addr)
		if typ != tt.wantType || name != tt.wantName || mod != tt.wantModule {
			t.Errorf("parseAddress(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.addr, typ, name, mod, tt.wantType, tt.wantName, tt.wantModule)
		}
	}
}

func TestProtoActionsToActions(t *testing.T) {
	tests := []struct {
		code    uint64
		want    []string
		wantErr bool
	}{
		{code: 0, want: []string{"no-op"}},
		{code: 1, want: []string{"create"}},
		{code: 2, want: []string{"read"}},
		{code: 3, want: []string{"update"}},
		{code: 5, want: []string{"delete"}},
		{code: 6, want: []string{"delete", "create"}},
		{code: 7, want: []string{"create", "delete"}},
		{code: 8, want: []string{"forget"}},
		{code: 9, want: []string{"open"}},
		{code: 4, wantErr: true},
		{code: 99, wantErr: true},
	}

	for _, tt := range tests {
		got, err := protoActionsToActions(tt.code)
		if tt.wantErr {
			if err == nil {
				t.Errorf("protoActionsToActions(%d) expected error, got nil", tt.code)
			}
			continue
		}
		if err != nil {
			t.Errorf("protoActionsToActions(%d) unexpected error: %v", tt.code, err)
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("protoActionsToActions(%d) = %v, want %v", tt.code, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("protoActionsToActions(%d)[%d] = %q, want %q", tt.code, i, got[i], tt.want[i])
			}
		}
	}
}

func createTestZipPlan(t *testing.T, addr string, actionCode uint64) []byte {
	t.Helper()

	var changeBytes []byte
	changeBytes = protowire.AppendTag(changeBytes, 1, protowire.VarintType)
	changeBytes = protowire.AppendVarint(changeBytes, actionCode)

	mpVal, err := msgpack.Marshal(map[string]interface{}{"name": "example"})
	if err != nil {
		t.Fatal(err)
	}
	var dvBytes []byte
	dvBytes = protowire.AppendTag(dvBytes, 1, protowire.BytesType)
	dvBytes = protowire.AppendBytes(dvBytes, mpVal)

	changeBytes = protowire.AppendTag(changeBytes, 2, protowire.BytesType)
	changeBytes = protowire.AppendBytes(changeBytes, dvBytes)

	var rcBytes []byte
	rcBytes = protowire.AppendTag(rcBytes, 13, protowire.BytesType)
	rcBytes = protowire.AppendString(rcBytes, addr)
	rcBytes = protowire.AppendTag(rcBytes, 9, protowire.BytesType)
	rcBytes = protowire.AppendBytes(rcBytes, changeBytes)

	var tfplanBytes []byte
	tfplanBytes = protowire.AppendTag(tfplanBytes, 3, protowire.BytesType)
	tfplanBytes = protowire.AppendBytes(tfplanBytes, rcBytes)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("tfplan")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(tfplanBytes); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func TestLoadBinaryPlanActions(t *testing.T) {
	tests := []struct {
		name       string
		actionCode uint64
		wantAction domain.ActionType
	}{
		{"noop", 0, domain.ActionNoOp},
		{"create", 1, domain.ActionCreate},
		{"read", 2, domain.ActionRead},
		{"update", 3, domain.ActionUpdate},
		{"delete", 5, domain.ActionDelete},
		{"delete_then_create", 6, domain.ActionReplace},
		{"create_then_delete", 7, domain.ActionReplace},
		{"forget", 8, domain.ActionForget},
		{"open", 9, domain.ActionOpen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zipBytes := createTestZipPlan(t, "module.test.aws_s3_bucket.example", tt.actionCode)
			plan, err := LoadReader(bytes.NewReader(zipBytes), "test.plan")
			if err != nil {
				t.Fatalf("LoadReader failed: %v", err)
			}
			if len(plan.ResourceChanges) != 1 {
				t.Fatalf("got %d resource changes, want 1", len(plan.ResourceChanges))
			}
			rc := plan.ResourceChanges[0]
			if rc.Address != "module.test.aws_s3_bucket.example" {
				t.Errorf("address = %q, want %q", rc.Address, "module.test.aws_s3_bucket.example")
			}
			if rc.Change.NormalizedAction() != tt.wantAction {
				t.Errorf("action = %v, want %v", rc.Change.NormalizedAction(), tt.wantAction)
			}
		})
	}
}

func TestLoadActionCode8Plan(t *testing.T) {
	planPath := "../../../data/action_code_8.plan"
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		t.Skip("skipping test: data/action_code_8.plan not present")
	}
	plan, err := Load(planPath)
	if err != nil {
		t.Fatalf("failed to load action_code_8.plan: %v", err)
	}
	var forgetCount int
	for _, rc := range plan.ResourceChanges {
		if rc.Change.NormalizedAction() == domain.ActionForget {
			forgetCount++
		}
	}
	if forgetCount == 0 {
		t.Fatalf("expected at least one resource with ActionForget, got 0")
	}
}

