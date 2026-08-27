package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ducva/tofu-diff/plan"
	"github.com/ducva/tofu-diff/render"
	"github.com/ducva/tofu-diff/tui"
	"golang.org/x/term"
)

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Fprintf(os.Stderr, "error: unexpected failure processing plan file: %v\n", rec)
			exitCode = 1
		}
	}()

	var diffOnly = true
	fs := flag.NewFlagSet("tofu-diff", flag.ContinueOnError)
	fs.BoolVar(&diffOnly, "diff-only", true, "show only changed lines/attributes (hide unchanged context)")
	fs.Usage = func() {
		fmt.Fprint(os.Stdout, `Usage: tofu-diff [--diff-only] [<plan-file>]

View the contents of an OpenTofu JSON plan file in a human-readable format.
The plan file must be produced with: tofu show -json <planfile>

Arguments:
  <plan-file>   Path to the JSON plan file (optional if piped via stdin)

Options:
  --diff-only   Show only diff part (hide unchanged context lines) (default true)
                Use --diff-only=false to show full context
                In TUI, press 'o' to toggle at runtime

Examples:
  tofu-diff plan.json
  tofu-diff --diff-only=false plan.json
  cat plan.json | tofu-diff
  tofu show -json tfplan | tofu-diff
`)
		os.Exit(0)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Detect if stdin is a pipe (data being piped in).
	stat, _ := os.Stdin.Stat()
	isPipe := (stat.Mode() & os.ModeCharDevice) == 0

	var pf *plan.PlanFile
	var loadErr error

	switch {
	case fs.NArg() >= 1:
		pf, loadErr = plan.Load(fs.Arg(0))
	case isPipe:
		pf, loadErr = plan.LoadReader(os.Stdin, "stdin")
	default:
		fmt.Fprintf(os.Stderr, "error: expected a <plan-file> argument or piped input\nRun 'tofu-diff --help' for usage.\n")
		return 1
	}

	if loadErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", loadErr)
		return 1
	}

	if term.IsTerminal(int(os.Stdout.Fd())) {
		model := tui.NewWithDiffOnly(*pf, diffOnly)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: TUI failed: %v\n", err)
			return 1
		}
		return 0
	}

	if err := render.NewWithDiffOnly(os.Stdout, diffOnly).Render(*pf); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}
