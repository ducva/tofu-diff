package domain

import "testing"

func TestNormalizeAction(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
		want    ActionType
		wantErr bool
	}{
		{name: "empty", want: ActionNoOp},
		{name: "create", actions: []string{"create"}, want: ActionCreate},
		{name: "update", actions: []string{"update"}, want: ActionUpdate},
		{name: "delete", actions: []string{"delete"}, want: ActionDelete},
		{name: "no-op", actions: []string{"no-op"}, want: ActionNoOp},
		{name: "read", actions: []string{"read"}, want: ActionNoOp},
		{name: "destroy then create", actions: []string{"delete", "create"}, want: ActionReplace},
		{name: "create then destroy", actions: []string{"create", "delete"}, want: ActionReplace},
		{name: "unknown", actions: []string{"move"}, wantErr: true},
		{name: "invalid pair", actions: []string{"update", "delete"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeAction(test.actions)
			if test.wantErr {
				if err == nil {
					t.Fatalf("NormalizeAction(%q) returned no error", test.actions)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeAction(%q): %v", test.actions, err)
			}
			if got != test.want {
				t.Fatalf("NormalizeAction(%q) = %q, want %q", test.actions, got, test.want)
			}
		})
	}
}
