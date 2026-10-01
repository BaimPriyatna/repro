// Package collector — environment variable collector.
//
// Privacy requirements:
//   - env vars are NOT captured by default
//   - callers must opt-in via EnvConfig.Enabled = true
//   - patterns in DenyList are always redacted, even when enabled
//   - redacted values are replaced with the sentinel "[REDACTED]" so the
//     variable's presence is still visible but its value is not
package collector

import (
	"context"
	"os"
	"strings"
)

const redactedSentinel = "[REDACTED]"

// EnvConfig controls what the EnvCollector captures.
type EnvConfig struct {
	// Enabled must be true for any env var to be captured.
	// Defaults to false — secrets must not be captured by default.
	Enabled bool

	// DenyList is a slice of glob-style patterns (case-insensitive, '*' and '?' supported).
	// Variables whose names match any pattern are always replaced with
	// "[REDACTED]", even when Enabled is true.
	// Example patterns: "*KEY*", "*SECRET*", "*TOKEN*"
	DenyList []string
}

// EnvInfo holds the captured environment variable map.
// Keys are variable names; values are either the raw value or "[REDACTED]".
type EnvInfo struct {
	// Captured is the filtered and (possibly) redacted env-var map.
	// Always sorted by key for deterministic output.
	Captured map[string]string `json:"captured"`

	// RedactedCount is the number of variables whose values were replaced.
	RedactedCount int `json:"redacted_count"`
}

// EnvCollector captures environment variables with privacy-safe defaults.
type EnvCollector struct {
	cfg EnvConfig
}

// NewEnvCollector returns an EnvCollector configured with cfg.
func NewEnvCollector(cfg EnvConfig) *EnvCollector {
	return &EnvCollector{cfg: cfg}
}

// Name implements Collector.
func (c *EnvCollector) Name() string { return "env" }

// Collect implements Collector.
// Returns an empty EnvInfo when Enabled is false — this is the safe default.
func (c *EnvCollector) Collect(_ context.Context) (Result, error) {
	if !c.cfg.Enabled {
		return Result{
			Data:    EnvInfo{Captured: map[string]string{}, RedactedCount: 0},
			Warning: "env var capture is disabled (privacy default); set EnvConfig.Enabled=true to capture",
		}, nil
	}

	raw := os.Environ()
	captured := make(map[string]string, len(raw))
	redactedCount := 0

	for _, kv := range raw {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue // malformed entry — skip silently
		}
		name := kv[:idx]
		value := kv[idx+1:]

		if c.isDenied(name) {
			captured[name] = redactedSentinel
			redactedCount++
		} else {
			captured[name] = value
		}
	}

	return Result{
		Data: EnvInfo{
			Captured:      captured,
			RedactedCount: redactedCount,
		},
	}, nil
}

// isDenied reports whether the variable name matches any deny-list pattern.
// Matching is case-insensitive.
func (c *EnvCollector) isDenied(name string) bool {
	upper := strings.ToUpper(name)
	for _, pattern := range c.cfg.DenyList {
		if matchGlob(strings.ToUpper(pattern), upper) {
			return true
		}
	}
	return false
}

// matchGlob performs a simple glob match supporting '*' (any sequence) and
// '?' (any single character) wildcards. This avoids external dependencies.
func matchGlob(pattern, s string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// Skip consecutive stars.
			for len(pattern) > 0 && pattern[0] == '*' {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true // trailing star matches everything
			}
			// Try matching the rest of the pattern at every position in s.
			for i := 0; i <= len(s); i++ {
				if matchGlob(pattern, s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(s) == 0 {
				return false
			}
			pattern = pattern[1:]
			s = s[1:]
		default:
			if len(s) == 0 || pattern[0] != s[0] {
				return false
			}
			pattern = pattern[1:]
			s = s[1:]
		}
	}
	return len(s) == 0
}
