package render

import (
  "bytes"
  "encoding/json"
  "testing"
  "github.com/ducva/tofu-diff/plan"
)

func TestRenderDiffOnly(t *testing.T) {
  before := json.RawMessage(`"old"`)
  after := json.RawMessage(`"new"`)
  unchangedBefore := json.RawMessage(`"same"`)
  unchangedAfter := json.RawMessage(`"same"`)
  rc := plan.ResourceChange{
    Address: "test.example",
    Change: plan.Change{
      Actions: []string{"update"},
      Before: map[string]json.RawMessage{"changed": before, "same": unchangedBefore},
      After: map[string]json.RawMessage{"changed": after, "same": unchangedAfter},
    },
  }
  pf := plan.PlanFile{FormatVersion: "1.0", ResourceChanges: []plan.ResourceChange{rc}}
  // diffOnly true should show only changed
  var buf bytes.Buffer
  r := NewWithDiffOnly(&buf, true)
  if err := r.Render(pf); err != nil { t.Fatal(err) }
  out := buf.String()
  if !contains(out, "changed") {
    t.Fatalf("diffOnly true should contain changed key, got %q", out)
  }
  if contains(out, "same") {
    t.Fatalf("diffOnly true should NOT contain unchanged key 'same', got %q", out)
  }
  // diffOnly false should show both
  buf.Reset()
  r2 := NewWithDiffOnly(&buf, false)
  if err := r2.Render(pf); err != nil { t.Fatal(err) }
  out2 := buf.String()
  if !contains(out2, "changed") || !contains(out2, "same") {
    t.Fatalf("diffOnly false should contain both keys, got %q", out2)
  }
  if !contains(out2, "    same =") {
    t.Fatalf("unchanged should be shown with '=' marker, got %q", out2)
  }
}

func contains(s, sub string) bool {
  return bytes.Contains([]byte(s), []byte(sub))
}
