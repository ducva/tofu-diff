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
