package airyra

import (
	"testing"
)

func TestPriorityFromString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		// Valid priorities
		{"critical", "critical", PriorityCritical, false},
		{"Critical uppercase", "Critical", PriorityCritical, false},
		{"CRITICAL all caps", "CRITICAL", PriorityCritical, false},
		{"high", "high", PriorityHigh, false},
		{"High mixed", "High", PriorityHigh, false},
		{"normal", "normal", PriorityNormal, false},
		{"empty string defaults to normal", "", PriorityNormal, false},
		{"low", "low", PriorityLow, false},
		{"lowest", "lowest", PriorityLowest, false},

		// Invalid priorities
		{"invalid string", "invalid", 0, true},
		{"number string", "1", 0, true},
		{"medium not valid", "medium", 0, true},
		{"urgent not valid", "urgent", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PriorityFromString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("PriorityFromString(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("PriorityFromString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPriorityToString(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  string
	}{
		{"critical (0)", PriorityCritical, "critical"},
		{"high (1)", PriorityHigh, "high"},
		{"normal (2)", PriorityNormal, "normal"},
		{"low (3)", PriorityLow, "low"},
		{"lowest (4)", PriorityLowest, "lowest"},
		{"unknown negative", -1, "unknown(-1)"},
		{"unknown high", 99, "unknown(99)"},
		{"unknown 5", 5, "unknown(5)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PriorityToString(tt.input)
			if got != tt.want {
				t.Errorf("PriorityToString(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPriorityRoundTrip(t *testing.T) {
	// Test that converting to string and back gives the same value
	priorities := []string{"critical", "high", "normal", "low", "lowest"}

	for _, p := range priorities {
		t.Run(p, func(t *testing.T) {
			val, err := PriorityFromString(p)
			if err != nil {
				t.Fatalf("PriorityFromString(%q) failed: %v", p, err)
			}

			str := PriorityToString(val)
			if str != p {
				t.Errorf("Round trip failed: %q -> %d -> %q", p, val, str)
			}
		})
	}
}

func TestPriorityConstants(t *testing.T) {
	// Verify priority ordering: critical < high < normal < low < lowest
	if PriorityCritical >= PriorityHigh {
		t.Error("PriorityCritical should be less than PriorityHigh")
	}
	if PriorityHigh >= PriorityNormal {
		t.Error("PriorityHigh should be less than PriorityNormal")
	}
	if PriorityNormal >= PriorityLow {
		t.Error("PriorityNormal should be less than PriorityLow")
	}
	if PriorityLow >= PriorityLowest {
		t.Error("PriorityLow should be less than PriorityLowest")
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify status constants are properly defined
	tests := []struct {
		status TaskStatus
		want   string
	}{
		{StatusOpen, "open"},
		{StatusInProgress, "in_progress"},
		{StatusBlocked, "blocked"},
		{StatusDone, "done"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("Status constant %v = %q, want %q", tt.status, string(tt.status), tt.want)
			}
		})
	}
}
