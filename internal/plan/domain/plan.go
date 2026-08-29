package domain

import "encoding/json"

// Change contains the before and after attribute states of one resource.
// The maps are kept presentation-neutral; unknown and sensitivity markers are
// domain facts populated by ingestion adapters.
type Change struct {
	Actions         []string
	Before          map[string]json.RawMessage
	After           map[string]json.RawMessage
	AfterUnknown    map[string]bool
	BeforeSensitive map[string]bool
	AfterSensitive  map[string]bool
}

// NormalizedAction returns the validated normalized action. Ingestion must call
// Validate before exposing a Plan, so the fallback is defensive only.
func (c Change) NormalizedAction() ActionType {
	action, err := NormalizeAction(c.Actions)
	if err != nil {
		return ActionNoOp
	}
	return action
}

func (c Change) Validate() error {
	_, err := NormalizeAction(c.Actions)
	return err
}

// ResourceChange is a proposed transition identified by its OpenTofu address.
type ResourceChange struct {
	Address       string
	ModuleAddress string
	Mode          string
	Type          string
	Name          string
	Change        Change
}

// Plan is the normalized, immutable-by-convention aggregate returned by an
// ingestion adapter. Presenters consume it without mutating domain semantics.
type Plan struct {
	FormatVersion   string
	ResourceChanges []ResourceChange
}

// Validate checks invariants which all ingestion adapters must satisfy.
func (p Plan) Validate() error {
	seen := make(map[string]struct{}, len(p.ResourceChanges))
	for _, rc := range p.ResourceChanges {
		if rc.Address == "" {
			return ErrMissingResourceAddress
		}
		if _, ok := seen[rc.Address]; ok {
			return DuplicateResourceAddressError{Address: rc.Address}
		}
		seen[rc.Address] = struct{}{}
		if err := rc.Change.Validate(); err != nil {
			return InvalidResourceChangeError{Address: rc.Address, Err: err}
		}
	}
	return nil
}
