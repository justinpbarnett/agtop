package safety

import (
	"fmt"
	"regexp"
	"strings"
)

type PatternMatcher struct {
	compiled []*regexp.Regexp
	raw      []string
}

// NewPatternMatcher compiles blocked patterns as case-insensitive regexes.
// Invalid patterns are skipped and collected into the returned error.
// The matcher is still usable with whichever patterns compiled successfully.
func NewPatternMatcher(patterns []string) (*PatternMatcher, error) {
	pm := &PatternMatcher{}
	var errs []string

	for i, p := range patterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			errs = append(errs, fmt.Sprintf("pattern[%d] %q: %v", i, p, err))
			continue
		}
		pm.compiled = append(pm.compiled, re)
		pm.raw = append(pm.raw, p)
	}

	if len(errs) > 0 {
		return pm, fmt.Errorf("invalid safety patterns: %s", strings.Join(errs, "; "))
	}
	return pm, nil
}

// Check tests a command string against all compiled patterns.
// Returns on first match.
func (pm *PatternMatcher) Check(command string) (blocked bool, matchedPattern string) {
	for i, re := range pm.compiled {
		if re.MatchString(command) {
			return true, pm.raw[i]
		}
	}
	return false, ""
}

// Patterns returns the original pattern strings that compiled successfully.
func (pm *PatternMatcher) Patterns() []string {
	out := make([]string, len(pm.raw))
	copy(out, pm.raw)
	return out
}

// PatternCount returns the number of successfully compiled patterns.
func (pm *PatternMatcher) PatternCount() int {
	return len(pm.compiled)
}
