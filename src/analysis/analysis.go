// Package analysis provides analyzers and shared diff/history helpers.
//
// Analyzers include BeforeAfter, Drift, Absent, and ChangeMap.
package analysis

import (
	"github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// Analyzer is the common interface for analysis modules.
type Analyzer interface {
	// Name returns the machine-readable identifier for this analyzer.
	Name() string

	// Version returns the version string for this analyzer implementation.
	Version() string

	// Analyze performs the analysis and returns a result.
	Analyze() (*analysis.AnalysisResult, error)
}

// SnapshotLoader abstracts snapshot retrieval for analyzers.
// This allows analyzers to work with different storage backends.
type SnapshotLoader interface {
	// Load retrieves a snapshot by ID.
	Load(id snapshot.ID) (*snapshot.Snapshot, error)

	// List retrieves multiple snapshots matching a filter.
	// Implementation depends on the underlying store.Filter type.
	List(filter interface{}) ([]*snapshot.Snapshot, error)
}

// ChangeType classifies the kind of change detected between two values.
type ChangeType string

const (
	// ChangeTypeAdded means the item exists in the new snapshot but not the old.
	ChangeTypeAdded ChangeType = "added"

	// ChangeTypeRemoved means the item exists in the old snapshot but not the new.
	ChangeTypeRemoved ChangeType = "removed"

	// ChangeTypeModified means the item exists in both but has different values.
	ChangeTypeModified ChangeType = "modified"

	// ChangeTypeUnchanged means the item exists in both with identical values.
	ChangeTypeUnchanged ChangeType = "unchanged"
)

// Change represents a single detected change in the snapshot data.
type Change struct {
	// Type classifies what kind of change this is.
	Type ChangeType `json:"type"`

	// Path is the dot-separated path to the changed field (e.g. "runtime.go.version").
	Path string `json:"path"`

	// OldValue is the value in the old/baseline snapshot (nil if added).
	OldValue any `json:"old_value,omitempty"`

	// NewValue is the value in the new/current snapshot (nil if removed).
	NewValue any `json:"new_value,omitempty"`
}

// DiffResult holds the output of comparing two snapshots.
type DiffResult struct {
	// SnapshotA is the first (older/baseline) snapshot ID.
	SnapshotA snapshot.ID `json:"snapshot_a"`

	// SnapshotB is the second (newer/current) snapshot ID.
	SnapshotB snapshot.ID `json:"snapshot_b"`

	// Changes is the list of detected changes.
	Changes []Change `json:"changes"`

	// Summary provides counts by change type.
	Summary DiffSummary `json:"summary"`
}

// DiffSummary provides aggregate counts of changes.
type DiffSummary struct {
	Added      int `json:"added"`
	Removed    int `json:"removed"`
	Modified   int `json:"modified"`
	Unchanged  int `json:"unchanged"`
	TotalItems int `json:"total_items"`
}

// HasChanges reports whether any changes were detected.
func (d *DiffResult) HasChanges() bool {
	return d.Summary.Added > 0 || d.Summary.Removed > 0 || d.Summary.Modified > 0
}

// HistoryQuery defines parameters for traversing snapshot history.
type HistoryQuery struct {
	// StartID is the snapshot to begin traversal from.
	StartID snapshot.ID

	// MaxDepth limits how far back in the parent chain to traverse (0 = unlimited).
	MaxDepth int

	// IncludeStart determines whether to include the starting snapshot in results.
	IncludeStart bool
}

// HistoryChain is an ordered sequence of snapshots linked by parent references.
type HistoryChain struct {
	// Snapshots is the ordered list from newest (index 0) to oldest.
	Snapshots []*snapshot.Snapshot `json:"snapshots"`

	// Depth is the length of the chain.
	Depth int `json:"depth"`
}

// IsEmpty reports whether the chain contains any snapshots.
func (h *HistoryChain) IsEmpty() bool {
	return len(h.Snapshots) == 0
}

// Oldest returns the oldest (last) snapshot in the chain, or nil if empty.
func (h *HistoryChain) Oldest() *snapshot.Snapshot {
	if len(h.Snapshots) == 0 {
		return nil
	}
	return h.Snapshots[len(h.Snapshots)-1]
}

// Newest returns the newest (first) snapshot in the chain, or nil if empty.
func (h *HistoryChain) Newest() *snapshot.Snapshot {
	if len(h.Snapshots) == 0 {
		return nil
	}
	return h.Snapshots[0]
}
