package ingestion

import (
	"encoding/json"
	"fmt"

	"github.com/ducva/tofu-diff/internal/plan/domain"
)

type jsonChange struct {
	Actions         []string                   `json:"actions"`
	Before          map[string]json.RawMessage `json:"before"`
	After           map[string]json.RawMessage `json:"after"`
	AfterUnknown    map[string]bool            `json:"after_unknown"`
	BeforeSensitive map[string]bool            `json:"before_sensitive"`
	AfterSensitive  map[string]bool            `json:"after_sensitive"`
}

type jsonResourceChange struct {
	Address       string     `json:"address"`
	ModuleAddress string     `json:"module_address"`
	Mode          string     `json:"mode"`
	Type          string     `json:"type"`
	Name          string     `json:"name"`
	Change        jsonChange `json:"change"`
}

type jsonPlan struct {
	FormatVersion   string               `json:"format_version"`
	ResourceChanges []jsonResourceChange `json:"resource_changes"`
}

func decodeJSON(data []byte, name string) (*domain.Plan, error) {
	var decoded jsonPlan
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("not a valid tofu JSON plan file from %s: %w", name, err)
	}

	plan := decoded.toDomain()
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tofu plan from %s: %w", name, err)
	}
	return &plan, nil
}

func (p jsonPlan) toDomain() domain.Plan {
	resources := make([]domain.ResourceChange, 0, len(p.ResourceChanges))
	for _, rc := range p.ResourceChanges {
		resources = append(resources, domain.ResourceChange{
			Address:       rc.Address,
			ModuleAddress: rc.ModuleAddress,
			Mode:          rc.Mode,
			Type:          rc.Type,
			Name:          rc.Name,
			Change: domain.Change{
				Actions:         rc.Change.Actions,
				Before:          rc.Change.Before,
				After:           rc.Change.After,
				AfterUnknown:    rc.Change.AfterUnknown,
				BeforeSensitive: rc.Change.BeforeSensitive,
				AfterSensitive:  rc.Change.AfterSensitive,
			},
		})
	}
	return domain.Plan{FormatVersion: p.FormatVersion, ResourceChanges: resources}
}
