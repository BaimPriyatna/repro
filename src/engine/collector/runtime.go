// Package collector — Go runtime version collector.
package collector

import (
	"context"
	"runtime"
)

// RuntimeInfo holds Go runtime metadata.
type RuntimeInfo struct {
	GoVersion string `json:"go_version"`
}

// RuntimeCollector captures the Go runtime version embedded in the binary.
// This is always safe to capture and cannot fail.
type RuntimeCollector struct{}

// NewRuntimeCollector returns a ready-to-use RuntimeCollector.
func NewRuntimeCollector() *RuntimeCollector { return &RuntimeCollector{} }

// Name implements Collector.
func (c *RuntimeCollector) Name() string { return "runtime" }

// Collect implements Collector.
// Returns the Go version string (e.g. "go1.27.1") — a compile-time constant.
func (c *RuntimeCollector) Collect(_ context.Context) (Result, error) {
	return Result{
		Data: RuntimeInfo{
			GoVersion: runtime.Version(),
		},
	}, nil
}
