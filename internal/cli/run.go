package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ducva/tofu-diff/internal/plan/application"
	"github.com/ducva/tofu-diff/internal/plan/ingestion"
	textpresenter "github.com/ducva/tofu-diff/internal/presentation/text"
	tuipresenter "github.com/ducva/tofu-diff/internal/presentation/tui"
	"golang.org/x/term"
)

func Main() int {
	stdinIsPipe := false
	if stdinStat, err := os.Stdin.Stat(); err == nil {
		stdinIsPipe = (stdinStat.Mode() & os.ModeCharDevice) == 0
	}
	stdoutIsTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	return Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, stdinIsPipe, stdoutIsTerminal)
}

// Run is the process-level adapter. Runtime characteristics are explicit so
// flag, source-selection, and plain-text behavior can be tested without a TTY.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, stdinIsPipe, stdoutIsTerminal bool) (exitCode int) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(stderr, "error: unexpected failure processing plan file: %v\n", recovered)
			exitCode = 1
		}
	}()

	diffOnly := true
	flags := flag.NewFlagSet("tofu-diff", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&diffOnly, "diff-only", true, "show only changed lines/attributes (hide unchanged context)")
	flags.Usage = func() { writeUsage(stdout) }

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	var source io.Reader
	sourceName := "stdin"
	var closeSource func() error

	switch {
	case flags.NArg() >= 1:
		file, err := os.Open(flags.Arg(0))
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(stderr, "error: file not found: %s\n", flags.Arg(0))
			} else {
				fmt.Fprintf(stderr, "error: cannot open file: %v\n", err)
			}
			return 1
		}
		source = file
		sourceName = flags.Arg(0)
		closeSource = file.Close
	case stdinIsPipe:
		source = stdin
	default:
		fmt.Fprintln(stderr, "error: expected a <plan-file> argument or piped input")
		fmt.Fprintln(stderr, "Run 'tofu-diff --help' for usage.")
		return 1
	}

	if closeSource != nil {
		defer closeSource()
	}

	useCase := application.InspectPlan{
		Decoder:     ingestion.Decoder{},
		Diagnostics: stderr,
	}

	var presenter application.Presenter
	if stdoutIsTerminal {
		presenter = tuipresenter.Presenter{DiffOnly: diffOnly, Input: stdin, Output: stdout}
	} else {
		presenter = textpresenter.NewWithDiffOnly(stdout, diffOnly)
	}

	if err := useCase.Execute(source, sourceName, presenter); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func writeUsage(out io.Writer) {
	fmt.Fprint(out, `Usage: tofu-diff [--diff-only] [<plan-file>]

View the contents of an OpenTofu JSON or native binary plan file in a human-readable format.

Arguments:
  <plan-file>   Path to a JSON or native plan file (optional if piped via stdin)

Options:
  --diff-only   Show only changed lines/attributes (default true)
                Use --diff-only=false to show full context
                In TUI, press 'o' to toggle at runtime

Examples:
  tofu-diff plan.json
  tofu-diff --diff-only=false plan.json
  cat plan.json | tofu-diff
  tofu show -json tfplan | tofu-diff
  tofu-diff tfplan
`)
}
