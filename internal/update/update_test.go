package update

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    int
	}{
		{"v0.1.0", "v0.2.0", -1},
		{"v1.0.0", "v1.0.0", 0},
		{"v2.0.0", "v1.0.0", 1},
		{"0.1.0", "v0.1.0", 0},         // mixed v prefix
		{"v0.1.0", "0.1.0", 0},         // mixed v prefix reversed
		{"0.1.0-3-gabcdef", "0.1.0", -1}, // git-describe prerelease < release
		{"0.2.0", "0.1.0-3-gabcdef", 1},  // release > prerelease
		{"dev", "v1.0.0", -1},           // unparseable current
		{"v1.0.0", "dev", 1},            // unparseable latest
		{"dev", "dev", 0},               // both unparseable
		{"v1.2.3", "v1.2.3", 0},        // exact match
		{"v0.0.1", "v0.0.2", -1},       // patch bump
		{"v0.1.0", "v0.0.9", 1},        // minor > patch
	}

	for _, tt := range tests {
		t.Run(tt.current+"_vs_"+tt.latest, func(t *testing.T) {
			got := CompareVersions(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestCheckForUpdateDevVersion(t *testing.T) {
	rel, err := CheckForUpdate("dev", "justinpbarnett/agtop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != nil {
		t.Errorf("expected nil release for dev version, got %+v", rel)
	}
}

func TestCheckForUpdateEmptyVersion(t *testing.T) {
	rel, err := CheckForUpdate("", "justinpbarnett/agtop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != nil {
		t.Errorf("expected nil release for empty version, got %+v", rel)
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"v1.0.0", false},
		{"1.0.0", false},
		{"0.1.0-3-gabcdef", false},
		{"v0.1.0-rc.1", false},
		{"dev", true},
		{"", true},
		{"not-a-version", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseSemver(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSemver(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
