package ingestion

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/ducva/tofu-diff/internal/plan/domain"
)

var zipMagic = []byte("PK\x03\x04")

// Load reads a named JSON or native plan file.
func Load(path string) (*domain.Plan, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()
	return LoadReader(file, path)
}

// LoadReader detects the external representation and delegates translation to
// the corresponding anti-corruption adapter.
func LoadReader(source io.Reader, name string) (*domain.Plan, error) {
	data, err := io.ReadAll(source)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", name, err)
	}

	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if bytes.HasPrefix(trimmed, zipMagic) {
		return loadBinaryBytes(data)
	}
	if !bytes.HasPrefix(trimmed, []byte("{")) {
		return nil, fmt.Errorf(
			"input from %s is not a valid JSON or binary tofu plan.\n\n"+
				"Hint: Did you pipe 'tofu plan' directly? That outputs human-readable text.\n"+
				"To get a JSON plan, use one of these patterns:\n"+
				"  tofu plan -out=tfplan && tofu show -json tfplan | tofu-diff\n"+
				"  tofu plan -out=tfplan && tofu show -json tfplan > plan.json && tofu-diff plan.json\n"+
				"  tofu-diff tfplan  (reads the binary plan file directly)",
			name,
		)
	}
	return decodeJSON(data, name)
}
