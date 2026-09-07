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

