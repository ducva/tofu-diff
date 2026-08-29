package domain

import (
	"encoding/json"
	"testing"
)

func TestDiffAttributesPreservesDomainMarkers(t *testing.T) {
	change := Change{
		Actions: []string{"update"},
		Before: map[string]json.RawMessage{
			"changed":   json.RawMessage(`"old"`),
			"same":      json.RawMessage(`"same"`),
			"computed":  json.RawMessage(`"known"`),
			"sensitive": json.RawMessage(`"secret"`),
		},
		After: map[string]json.RawMessage{
			"changed":   json.RawMessage(`"new"`),
			"same":      json.RawMessage(`"same"`),
			"computed":  json.RawMessage(`null`),
			"sensitive": json.RawMessage(`"new-secret"`),
		},
		AfterUnknown:    map[string]bool{"computed": true},
		BeforeSensitive: map[string]bool{"sensitive": true},
		AfterSensitive:  map[string]bool{"sensitive": true},
	}

	diffs := DiffAttributes(change)
	if len(diffs) != 3 {
		t.Fatalf("DiffAttributes returned %d diffs, want 3", len(diffs))
	}
	if diffs[0].Key != "changed" || diffs[1].Key != "computed" || diffs[2].Key != "sensitive" {
		t.Fatalf("diffs are not sorted: %#v", diffs)
	}
	if !diffs[1].IsUnknownAfter {
		t.Fatal("computed attribute lost unknown-after-apply marker")
	}
	if !diffs[2].BeforeSensitive || !diffs[2].AfterSensitive {
		t.Fatal("sensitive attribute lost sensitivity markers")
	}
}
