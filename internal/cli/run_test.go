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
	if !strings.Contains(stdout.String(), "--group-by-module") {
		t.Fatalf("help output missing --group-by-module: %q", stdout.String())
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

func TestRunGroupByModulePipedJSON(t *testing.T) {
	input := `{
		"format_version":"1.0",
		"resource_changes":[
			{
				"address":"module.vpc.aws_subnet.public",
				"module_address":"module.vpc",
				"change":{"actions":["create"],"after":{"cidr_block":"10.0.1.0/24"}}
			},
			{
				"address":"aws_s3_bucket.main",
				"change":{"actions":["create"],"after":{"bucket":"my-bucket"}}
			}
		]
	}`
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--group-by-module"}, strings.NewReader(input), &stdout, &stderr, true, false); code != 0 {
		t.Fatalf("Run returned %d, want 0: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Module: (root)") || !strings.Contains(out, "Module: module.vpc") {
		t.Fatalf("expected grouped modules in output, got:\n%s", out)
	}
}

