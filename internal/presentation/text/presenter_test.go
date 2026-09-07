package text

import (
  "bytes"
  "encoding/json"
  "testing"
  "github.com/ducva/tofu-diff/internal/plan/domain"
)

func TestRenderDiffOnly(t *testing.T) {
  before := json.RawMessage(`"old"`)
  after := json.RawMessage(`"new"`)
  unchangedBefore := json.RawMessage(`"same"`)
  unchangedAfter := json.RawMessage(`"same"`)
  rc := domain.ResourceChange{
    Address: "test.example",
    Change: domain.Change{
      Actions: []string{"update"},
      Before: map[string]json.RawMessage{"changed": before, "same": unchangedBefore},
      After: map[string]json.RawMessage{"changed": after, "same": unchangedAfter},
    },
  }
  pf := domain.Plan{FormatVersion: "1.0", ResourceChanges: []domain.ResourceChange{rc}}
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

func TestRenderGroupByModule(t *testing.T) {
	rcRoot := domain.ResourceChange{
		Address: "aws_s3_bucket.main",
		Change: domain.Change{
			Actions: []string{"create"},
			After:   map[string]json.RawMessage{"bucket": json.RawMessage(`"my-bucket"`)},
		},
	}
	rcVpc := domain.ResourceChange{
		Address:       "module.vpc.aws_subnet.public",
		ModuleAddress: "module.vpc",
		Change: domain.Change{
			Actions: []string{"update"},
			Before:  map[string]json.RawMessage{"cidr_block": json.RawMessage(`"10.0.1.0/24"`)},
			After:   map[string]json.RawMessage{"cidr_block": json.RawMessage(`"10.0.2.0/24"`)},
		},
	}
	rcAuth := domain.ResourceChange{
		Address:       "module.auth.aws_cognito.pool",
		ModuleAddress: "module.auth",
		Change: domain.Change{
			Actions: []string{"create"},
			After:   map[string]json.RawMessage{"name": json.RawMessage(`"users"`)},
		},
	}

	pf := domain.Plan{
		FormatVersion:   "1.0",
		ResourceChanges: []domain.ResourceChange{rcVpc, rcRoot, rcAuth},
	}

	var buf bytes.Buffer
	r := NewWithOptions(&buf, true, true)
	if err := r.Render(pf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Check headers exist
	if !contains(out, "Module: (root)") {
		t.Fatalf("expected Module: (root) header, got:\n%s", out)
	}
	if !contains(out, "Module: module.auth") {
		t.Fatalf("expected Module: module.auth header, got:\n%s", out)
	}
	if !contains(out, "Module: module.vpc") {
		t.Fatalf("expected Module: module.vpc header, got:\n%s", out)
	}

	// Verify order: (root) -> module.auth -> module.vpc
	idxRoot := bytes.Index([]byte(out), []byte("Module: (root)"))
	idxAuth := bytes.Index([]byte(out), []byte("Module: module.auth"))
	idxVpc := bytes.Index([]byte(out), []byte("Module: module.vpc"))

	if !(idxRoot < idxAuth && idxAuth < idxVpc) {
		t.Fatalf("expected order (root) < module.auth < module.vpc, got indices %d, %d, %d",
			idxRoot, idxAuth, idxVpc)
	}

	// Verify ungrouped does not have Module: headers
	buf.Reset()
	r.SetGroupByModule(false)
	if err := r.Render(pf); err != nil {
		t.Fatal(err)
	}
	outUngrouped := buf.String()
	if contains(outUngrouped, "Module: ") {
		t.Fatalf("ungrouped output should not contain 'Module: ' header, got:\n%s", outUngrouped)
	}
}

