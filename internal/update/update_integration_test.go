//go:build integration

package update

import "testing"

func TestCheckForUpdateIntegration(t *testing.T) {
	// Uses a very old version to ensure an update is always found.
	rel, err := CheckForUpdate("0.0.1", "justinpbarnett/agtop")
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if rel == nil {
		t.Fatal("expected a release to be available for v0.0.1, got nil")
	}
	if rel.Version == "" {
		t.Error("release version is empty")
	}
	t.Logf("latest release: v%s", rel.Version)
}
