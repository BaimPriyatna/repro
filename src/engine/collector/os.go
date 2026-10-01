// Package collector — OS info collector.
package collector

import (
	"context"
	"runtime"
)

// OSInfo holds operating system and architecture metadata.
type OSInfo struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// OSCollector captures the operating system and CPU architecture of the host.
// This is always safe to capture — it contains no secrets.
type OSCollector struct{}

// NewOSCollector returns a ready-to-use OSCollector.
func NewOSCollector() *OSCollector { return &OSCollector{} }

// Name implements Collector.
func (c *OSCollector) Name() string { return "os" }

// Collect implements Collector.
// Returns GOOS and GOARCH — these are compile-time constants embedded in the
// binary, so collection is always deterministic and cannot fail.
func (c *OSCollector) Collect(_ context.Context) (Result, error) {
	return Result{
		Data: OSInfo{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
	}, nil
}
