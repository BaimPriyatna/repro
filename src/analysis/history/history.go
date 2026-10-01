// Package history provides snapshot parent-chain traversal and timeline queries.
package history

import (
	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// Traverser walks snapshot history chains.
type Traverser struct {
	loader analysis.SnapshotLoader
}

// NewTraverser creates a new history traverser with the given snapshot loader.
func NewTraverser(loader analysis.SnapshotLoader) *Traverser {
	return &Traverser{loader: loader}
}

// Traverse walks the parent chain starting from the given snapshot ID.
// Returns a HistoryChain ordered from newest (index 0) to oldest.
func (t *Traverser) Traverse(query analysis.HistoryQuery) (*analysis.HistoryChain, error) {
	if query.StartID == "" {
		return nil, analysis.ErrInvalidInput
	}

	chain := &analysis.HistoryChain{
		Snapshots: make([]*snapshot.Snapshot, 0),
	}

	// Load the starting snapshot
	current, err := t.loader.Load(query.StartID)
	if err != nil {
		return nil, err
	}

	// Track visited IDs to detect cycles
	visited := make(map[snapshot.ID]bool)

	// Include the starting snapshot if requested
	if query.IncludeStart {
		chain.Snapshots = append(chain.Snapshots, current)
		visited[current.ID] = true
		chain.Depth++
	}

	// Walk the parent chain
	for {
		// Check depth limit
		if query.MaxDepth > 0 && len(chain.Snapshots) >= query.MaxDepth {
			break
		}

		// Check if current snapshot has a parent
		if current.ParentID == "" {
			break // reached the root
		}

		// Cycle detection
		if visited[current.ParentID] {
			// Defensive: parent chain should never have cycles, but protect against it
			break
		}

		// Load parent
		parent, err := t.loader.Load(current.ParentID)
		if err != nil {
			// Parent not found - stop traversal but return what we have
			break
		}

		chain.Snapshots = append(chain.Snapshots, parent)
		visited[parent.ID] = true
		chain.Depth++

		current = parent
	}

	if len(chain.Snapshots) == 0 {
		return nil, analysis.ErrEmptyHistory
	}

	return chain, nil
}

// GetAncestor retrieves a specific ancestor N steps back from the starting snapshot.
// N=1 returns the immediate parent, N=2 returns the grandparent, etc.
// Returns nil if the ancestor does not exist.
func (t *Traverser) GetAncestor(startID snapshot.ID, n int) (*snapshot.Snapshot, error) {
	if n < 1 {
		return nil, analysis.ErrInvalidInput
	}

	chain, err := t.Traverse(analysis.HistoryQuery{
		StartID:      startID,
		MaxDepth:     n,
		IncludeStart: false, // we want ancestors, not the start
	})
	if err != nil {
		return nil, err
	}

	if chain.Depth < n {
		return nil, analysis.ErrInsufficientData
	}

	// chain.Snapshots[0] is the immediate parent (N=1)
	// chain.Snapshots[1] is the grandparent (N=2), etc.
	return chain.Snapshots[n-1], nil
}

// GetParent retrieves the immediate parent of the given snapshot.
// This is a convenience wrapper for GetAncestor(id, 1).
func (t *Traverser) GetParent(id snapshot.ID) (*snapshot.Snapshot, error) {
	return t.GetAncestor(id, 1)
}

// FindCommonAncestor finds the most recent common ancestor of two snapshots.
// Returns nil if no common ancestor exists.
func (t *Traverser) FindCommonAncestor(idA, idB snapshot.ID) (*snapshot.Snapshot, error) {
	if idA == "" || idB == "" {
		return nil, analysis.ErrInvalidInput
	}

	// If they're the same snapshot, that's the common ancestor
	if idA == idB {
		snap, err := t.loader.Load(idA)
		if err != nil {
			return nil, err
		}
		return snap, nil
	}

	// Traverse both chains
	chainA, err := t.Traverse(analysis.HistoryQuery{
		StartID:      idA,
		IncludeStart: true,
	})
	if err != nil {
		return nil, err
	}

	chainB, err := t.Traverse(analysis.HistoryQuery{
		StartID:      idB,
		IncludeStart: true,
	})
	if err != nil {
		return nil, err
	}

	// Build a set of all ancestors of A
	ancestorsA := make(map[snapshot.ID]*snapshot.Snapshot)
	for _, snap := range chainA.Snapshots {
		ancestorsA[snap.ID] = snap
	}

	// Find the first snapshot in B's chain that appears in A's ancestors
	for _, snap := range chainB.Snapshots {
		if common, found := ancestorsA[snap.ID]; found {
			return common, nil
		}
	}

	// No common ancestor
	return nil, nil
}

// TraverseMultiple retrieves history chains for multiple snapshot IDs.
// Returns a map of snapshot ID to its history chain.
// Useful for batch operations like drift detection across multiple snapshots.
func (t *Traverser) TraverseMultiple(queries []analysis.HistoryQuery) (map[snapshot.ID]*analysis.HistoryChain, error) {
	results := make(map[snapshot.ID]*analysis.HistoryChain)

	for _, query := range queries {
		chain, err := t.Traverse(query)
		if err != nil {
			// Store error in results as nil chain
			results[query.StartID] = nil
			continue
		}
		results[query.StartID] = chain
	}

	return results, nil
}

// GetSnapshotsBetween retrieves all snapshots between two snapshot IDs (inclusive).
// The snapshots must be on the same parent chain, otherwise returns an error.
// Returns snapshots ordered from newer to older.
func (t *Traverser) GetSnapshotsBetween(newerID, olderID snapshot.ID) ([]*snapshot.Snapshot, error) {
	if newerID == "" || olderID == "" {
		return nil, analysis.ErrInvalidInput
	}

	if newerID == olderID {
		snap, err := t.loader.Load(newerID)
		if err != nil {
			return nil, err
		}
		return []*snapshot.Snapshot{snap}, nil
	}

	// Traverse from the newer snapshot
	chain, err := t.Traverse(analysis.HistoryQuery{
		StartID:      newerID,
		IncludeStart: true,
	})
	if err != nil {
		return nil, err
	}

	// Find the older snapshot in the chain
	result := make([]*snapshot.Snapshot, 0)
	found := false

	for _, snap := range chain.Snapshots {
		result = append(result, snap)
		if snap.ID == olderID {
			found = true
			break
		}
	}

	if !found {
		return nil, analysis.ErrInsufficientData
	}

	return result, nil
}

// CountGenerations returns the number of generations (parent hops) between
// two snapshots. Returns -1 if the snapshots are not on the same chain.
func (t *Traverser) CountGenerations(newerID, olderID snapshot.ID) (int, error) {
	snapshots, err := t.GetSnapshotsBetween(newerID, olderID)
	if err != nil {
		return -1, err
	}

	// Number of generations is one less than the number of snapshots
	return len(snapshots) - 1, nil
}
