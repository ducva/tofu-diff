package ingestion

import (
	"strings"
	"testing"

	"github.com/ducva/tofu-diff/internal/plan/domain"
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

func TestLoadActionCode8Plan(t *testing.T) {
	plan, err := Load("../../../data/action_code_8.plan")
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

