package application

import (
	"fmt"
	"io"

	"github.com/ducva/tofu-diff/internal/plan/domain"
)

// Decoder is the input port implemented by OpenTofu ingestion adapters.
type Decoder interface {
	Decode(source io.Reader, name string) (*domain.Plan, error)
}

// Presenter is the output port implemented by text and interactive adapters.
type Presenter interface {
	Present(domain.Plan) error
}

// InspectPlan coordinates decoding and presentation without knowing either
// the source encoding or terminal implementation.
type InspectPlan struct {
	Decoder     Decoder
	Diagnostics io.Writer
}

func (uc InspectPlan) Execute(source io.Reader, name string, presenter Presenter) error {
	plan, err := uc.Decoder.Decode(source, name)
	if err != nil {
		return err
	}
	if plan.FormatVersion != "1.0" && uc.Diagnostics != nil {
		fmt.Fprintf(uc.Diagnostics, "warning: unrecognized plan format_version %q; output may be incorrect\n", plan.FormatVersion)
	}
	if err := presenter.Present(*plan); err != nil {
		return fmt.Errorf("present plan: %w", err)
	}
	return nil
}
