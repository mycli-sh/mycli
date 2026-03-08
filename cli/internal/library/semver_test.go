package library

import "testing"

func TestParseSemver(t *testing.T) {
	tests := []struct {
		tag                 string
		major, minor, patch int
		wantErr             bool
	}{
		{"v0.0.1", 0, 0, 1, false},
		{"v1.2.3", 1, 2, 3, false},
		{"v10.20.30", 10, 20, 30, false},
		{"v0.0.0", 0, 0, 0, false},
		{"1.2.3", 0, 0, 0, true},    // missing v prefix
		{"v1.2", 0, 0, 0, true},     // only two parts
		{"v1.2.3.4", 0, 0, 0, true}, // too many parts
		{"va.b.c", 0, 0, 0, true},   // non-numeric
		{"", 0, 0, 0, true},
	}

	for _, tt := range tests {
		major, minor, patch, err := ParseSemver(tt.tag)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseSemver(%q) expected error", tt.tag)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSemver(%q) unexpected error: %v", tt.tag, err)
			continue
		}
		if major != tt.major || minor != tt.minor || patch != tt.patch {
			t.Errorf("ParseSemver(%q) = (%d, %d, %d), want (%d, %d, %d)",
				tt.tag, major, minor, patch, tt.major, tt.minor, tt.patch)
		}
	}
}

func TestFormatTag(t *testing.T) {
	if got := FormatTag(1, 2, 3); got != "v1.2.3" {
		t.Errorf("FormatTag(1, 2, 3) = %q, want v1.2.3", got)
	}
	if got := FormatTag(0, 0, 0); got != "v0.0.0" {
		t.Errorf("FormatTag(0, 0, 0) = %q, want v0.0.0", got)
	}
}

func TestBumpSemver(t *testing.T) {
	tests := []struct {
		tag, part string
		want      string
		wantErr   bool
	}{
		{"v0.0.6", "patch", "v0.0.7", false},
		{"v0.0.6", "minor", "v0.1.0", false},
		{"v0.0.6", "major", "v1.0.0", false},
		{"v1.2.3", "patch", "v1.2.4", false},
		{"v1.2.3", "minor", "v1.3.0", false},
		{"v1.2.3", "major", "v2.0.0", false},
		{"v1.2.3", "invalid", "", true},
		{"bad", "patch", "", true},
	}

	for _, tt := range tests {
		got, err := BumpSemver(tt.tag, tt.part)
		if tt.wantErr {
			if err == nil {
				t.Errorf("BumpSemver(%q, %q) expected error", tt.tag, tt.part)
			}
			continue
		}
		if err != nil {
			t.Errorf("BumpSemver(%q, %q) unexpected error: %v", tt.tag, tt.part, err)
			continue
		}
		if got != tt.want {
			t.Errorf("BumpSemver(%q, %q) = %q, want %q", tt.tag, tt.part, got, tt.want)
		}
	}
}
