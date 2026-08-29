package main

import (
	"os"

	"github.com/ducva/tofu-diff/internal/cli"
)

// Keep the module root installable with
// `go install github.com/ducva/tofu-diff@latest` while the canonical command
// implementation lives under cmd/tofu-diff.
func main() {
	os.Exit(cli.Main())
}
