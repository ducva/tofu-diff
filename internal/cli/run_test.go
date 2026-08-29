package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr, false, false); code != 0 {
		t.Fatalf("Run returned %d, want 0: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: tofu-diff") {
		t.Fatalf("help output missing usage: %q", stdout.String())
	}
}

func TestRunPipedJSON(t *testing.T) {
	input := `{"format_version":"1.0","resource_changes":[]}`
	var stdout, stderr bytes.Buffer
	if code := Run(nil, strings.NewReader(input), &stdout, &stderr, true, false); code != 0 {
		t.Fatalf("Run returned %d, want 0: %s", code, stderr.String())
	}
	if got := stdout.String(); got != "No changes. Infrastructure is up-to-date.\n" {
		t.Fatalf("unexpected output %q", got)
	}
}
