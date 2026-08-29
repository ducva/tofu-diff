package tui

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ducva/tofu-diff/internal/plan/domain"
)

// Presenter adapts the Bubble Tea interface to the application output port.
type Presenter struct {
	DiffOnly bool
	Input    io.Reader
	Output   io.Writer
}

func (p Presenter) Present(plan domain.Plan) error {
	model := NewWithDiffOnly(plan, p.DiffOnly)
	options := []tea.ProgramOption{tea.WithAltScreen()}
	if p.Input != nil {
		options = append(options, tea.WithInput(p.Input))
	}
	if p.Output != nil {
		options = append(options, tea.WithOutput(p.Output))
	}
	program := tea.NewProgram(model, options...)
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("TUI failed: %w", err)
	}
	return nil
}
