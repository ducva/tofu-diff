package ingestion

import (
	"io"

	"github.com/ducva/tofu-diff/internal/plan/domain"
)

// Decoder implements the application input port for JSON and native plans.
type Decoder struct{}

func (Decoder) Decode(source io.Reader, name string) (*domain.Plan, error) {
	return LoadReader(source, name)
}
