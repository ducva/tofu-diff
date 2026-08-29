package domain

import "testing"

func TestPlanValidateRejectsDuplicateAddress(t *testing.T) {
	plan := Plan{ResourceChanges: []ResourceChange{
		{Address: "test.example", Change: Change{Actions: []string{"create"}}},
		{Address: "test.example", Change: Change{Actions: []string{"delete"}}},
	}}
	if err := plan.Validate(); err == nil {
		t.Fatal("Validate accepted duplicate resource addresses")
	}
}
