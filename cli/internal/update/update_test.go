package update

import "testing"

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"v0.1.0", "0.1.0"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeVersion(tt.input); got != tt.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int // >0, <0, or 0
	}{
		{"0.6.0", "0.5.0", 1},
		{"0.5.0", "0.6.0", -1},
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "0.9.9", 1},
		{"0.10.0", "0.9.0", 1},
		{"2.0.0", "1.99.99", 1},
		{"1.0", "1.0.0", 0},
		{"1.0.1", "1.0", 1},
	}
	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		switch {
		case tt.want > 0 && got <= 0:
			t.Errorf("compareVersions(%q, %q) = %d, want >0", tt.a, tt.b, got)
		case tt.want < 0 && got >= 0:
			t.Errorf("compareVersions(%q, %q) = %d, want <0", tt.a, tt.b, got)
		case tt.want == 0 && got != 0:
			t.Errorf("compareVersions(%q, %q) = %d, want 0", tt.a, tt.b, got)
		}
	}
}
