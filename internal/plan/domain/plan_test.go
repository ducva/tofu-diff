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

func TestResourceChangeModuleName(t *testing.T) {
	tests := []struct {
		name     string
		rc       ResourceChange
		expected string
	}{
		{
			name:     "root module without module address",
			rc:       ResourceChange{Address: "aws_s3_bucket.example"},
			expected: "(root)",
		},
		{
			name:     "explicit module address",
			rc:       ResourceChange{Address: "module.vpc.aws_subnet.public", ModuleAddress: "module.vpc"},
			expected: "module.vpc",
		},
		{
			name:     "implicit module address from address",
			rc:       ResourceChange{Address: "module.vpc.aws_subnet.public"},
			expected: "module.vpc",
		},
		{
			name:     "nested module address",
			rc:       ResourceChange{Address: "module.vpc.module.subnet.aws_subnet.public"},
			expected: "module.vpc.module.subnet",
		},
		{
			name:     "indexed module address",
			rc:       ResourceChange{Address: "module.vpc[0].aws_subnet.public"},
			expected: "module.vpc[0]",
		},
		{
			name:     "indexed module and indexed resource",
			rc:       ResourceChange{Address: "module.vpc[0].aws_subnet.public[1]"},
			expected: "module.vpc[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rc.ModuleName(); got != tt.expected {
				t.Errorf("ModuleName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestResourceChangeRelativeAddress(t *testing.T) {
	tests := []struct {
		name     string
		rc       ResourceChange
		expected string
	}{
		{
			name:     "root module keeps full address",
			rc:       ResourceChange{Address: "aws_s3_bucket.example"},
			expected: "aws_s3_bucket.example",
		},
		{
			name:     "module prefix stripped",
			rc:       ResourceChange{Address: "module.vpc.aws_subnet.public", ModuleAddress: "module.vpc"},
			expected: "aws_subnet.public",
		},
		{
			name:     "nested module prefix stripped",
			rc:       ResourceChange{Address: "module.vpc.module.subnet.aws_subnet.public", ModuleAddress: "module.vpc.module.subnet"},
			expected: "aws_subnet.public",
		},
		{
			name:     "indexed module prefix stripped",
			rc:       ResourceChange{Address: "module.vpc[0].aws_subnet.public", ModuleAddress: "module.vpc[0]"},
			expected: "aws_subnet.public",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rc.RelativeAddress(); got != tt.expected {
				t.Errorf("RelativeAddress() = %q, want %q", got, tt.expected)
			}
		})
	}
}

