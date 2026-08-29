package application

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ducva/tofu-diff/internal/plan/domain"
)

type stubDecoder struct{ plan *domain.Plan }

func (d stubDecoder) Decode(io.Reader, string) (*domain.Plan, error) { return d.plan, nil }

type recordingPresenter struct{ presented bool }

func (p *recordingPresenter) Present(domain.Plan) error {
	p.presented = true
	return nil
}

func TestInspectPlanReportsVersionDiagnosticAndPresents(t *testing.T) {
	var diagnostics bytes.Buffer
	presenter := &recordingPresenter{}
	useCase := InspectPlan{
		Decoder:     stubDecoder{plan: &domain.Plan{FormatVersion: "2.0"}},
		Diagnostics: &diagnostics,
	}

	if err := useCase.Execute(strings.NewReader("ignored"), "fixture", presenter); err != nil {
		t.Fatal(err)
	}
	if !presenter.presented {
		t.Fatal("presenter was not called")
	}
	if !strings.Contains(diagnostics.String(), `format_version "2.0"`) {
		t.Fatalf("missing version diagnostic: %q", diagnostics.String())
	}
}
