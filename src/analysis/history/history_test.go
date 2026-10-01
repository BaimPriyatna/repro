package history_test

import (
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/history"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// mockLoader implements analysis.SnapshotLoader for testing.
type mockLoader struct {
	snapshots map[snapshot.ID]*snapshot.Snapshot
}

func (m *mockLoader) Load(id snapshot.ID) (*snapshot.Snapshot, error) {
	snap, ok := m.snapshots[id]
	if !ok {
		return nil, analysis.ErrSnapshotNotFound
	}
	return snap, nil
}

func (m *mockLoader) List(filter interface{}) ([]*snapshot.Snapshot, error) {
	result := make([]*snapshot.Snapshot, 0, len(m.snapshots))
	for _, snap := range m.snapshots {
		result = append(result, snap)
	}
	return result, nil
}

// createLinearChain creates a linear parent chain: newest ->... -> oldest.
func createLinearChain(length int) (map[snapshot.ID]*snapshot.Snapshot, []snapshot.ID) {
	snapshots := make(map[snapshot.ID]*snapshot.Snapshot)
	ids := make([]snapshot.ID, length)

	var parentID snapshot.ID
	for i := length - 1; i >= 0; i-- {
		id := snapshot.NewID()
		snap := &snapshot.Snapshot{
			ID:        id,
			ParentID:  parentID,
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Data:      map[string]any{"index": i},
		}
		snapshots[id] = snap
		ids[i] = id
		parentID = id
	}

	return snapshots, ids
}

func TestTraverse_BasicChain(t *testing.T) {
	snapshots, ids := createLinearChain(5)
	loader := &mockLoader{snapshots: snapshots}
	traverser := history.NewTraverser(loader)

	// Traverse from newest (ids[0])
	chain, err := traverser.Traverse(analysis.HistoryQuery{
		StartID:      ids[0],
		IncludeStart: true,
	})

	if err != nil {
		t.Fatalf("Traverse() error = %v", err)
	}

	if chain.Depth != 5 {
		t.Errorf("Depth = %d, want 5", chain.Depth)
	}

	if len(chain.Snapshots) != 5 {
		t.Errorf("len(Snapshots) = %d, want 5", len(chain.Snapshots))
	}

	// Verify ordering (newest to oldest)
	for i := 0; i < len(chain.Snapshots); i++ {
		if chain.Snapshots[i].ID != ids[i] {
			t.Errorf("Snapshots[%d].ID = %v, want %v", i, chain.Snapshots[i].ID, ids[i])
		}
	}
}

func TestTraverse_MaxDepth(t *testing.T) {
	snapshots, ids := createLinearChain(10)
	loader := &mockLoader{snapshots: snapshots}
	traverser := history.NewTraverser(loader)

	chain, err := traverser.Traverse(analysis.HistoryQuery{
		StartID:      ids[0],
		MaxDepth:     3,
		IncludeStart: true,
	})

	if err != nil {
		t.Fatalf("Traverse() error = %v", err)
	}

	if chain.Depth != 3 {
		t.Errorf("Depth = %d, want 3", chain.Depth)
	}

	if len(chain.Snapshots) != 3 {
		t.Errorf("len(Snapshots) = %d, want 3", len(chain.Snapshots))
	}
}

func TestTraverse_ExcludeStart(t *testing.T) {
	snapshots, ids := createLinearChain(5)
	loader := &mockLoader{snapshots: snapshots}
	traverser := history.NewTraverser(loader)

	chain, err := traverser.Traverse(analysis.HistoryQuery{
		StartID:      ids[0],
		IncludeStart: false, // exclude the starting snapshot
	})

	if err != nil {
		t.Fatalf("Traverse() error = %v", err)
	}

	// Should have 4 snapshots (excluding the start)
	if len(chain.Snapshots) != 4 {
		t.Errorf("len(Snapshots) = %d, want 4", len(chain.Snapshots))
	}

	// First snapshot should be the parent of ids[0]
	if chain.Snapshots[0].ID != ids[1] {
		t.Errorf("Snapshots[0].ID = %v, want %v (parent of start)", chain.Snapshots[0].ID, ids[1])
	}
}

func TestTraverse_SingleSnapshot(t *testing.T) {
	id := snapshot.NewID()
	snap := &snapshot.Snapshot{
		ID:       id,
		ParentID: "", // no parent
		Data:     map[string]any{},
	}

	loader := &mockLoader{
		snapshots: map[snapshot.ID]*snapshot.Snapshot{
			id: snap,
		},
	}
	traverser := history.NewTraverser(loader)

	chain, err := traverser.Traverse(analysis.HistoryQuery{
		StartID:      id,
		IncludeStart: true,
	})

	if err != nil {
		t.Fatalf("Traverse() error = %v", err)
	}

	if chain.Depth != 1 {
		t.Errorf("Depth = %d, want 1", chain.Depth)
	}
}

func TestTraverse_InvalidInput(t *testing.T) {
	loader := &mockLoader{snapshots: make(map[snapshot.ID]*snapshot.Snapshot)}
	traverser := history.NewTraverser(loader)

	_, err := traverser.Traverse(analysis.HistoryQuery{
		StartID: "", // invalid
	})

	if err == nil {
		t.Error("Traverse with empty StartID should return error")
	}
}

func TestTraverse_NotFound(t *testing.T) {
	loader := &mockLoader{snapshots: make(map[snapshot.ID]*snapshot.Snapshot)}
	traverser := history.NewTraverser(loader)

	_, err := traverser.Traverse(analysis.HistoryQuery{
		StartID:      snapshot.NewID(), // doesn't exist
		IncludeStart: true,
	})

	if err == nil {
		t.Error("Traverse with non-existent snapshot should return error")
	}
}

func TestGetAncestor(t *testing.T) {
	snapshots, ids := createLinearChain(5)
	loader := &mockLoader{snapshots: snapshots}
	traverser := history.NewTraverser(loader)

	// Get immediate parent (N=1)
	parent, err := traverser.GetAncestor(ids[0], 1)
	if err != nil {
		t.Fatalf("GetAncestor(1) error = %v", err)
	}
	if parent.ID != ids[1] {
		t.Errorf("GetAncestor(1) = %v, want %v", parent.ID, ids[1])
	}

	// Get grandparent (N=2)
	grandparent, err := traverser.GetAncestor(ids[0], 2)
	if err != nil {
		t.Fatalf("GetAncestor(2) error = %v", err)
	}
	if grandparent.ID != ids[2] {
		t.Errorf("GetAncestor(2) = %v, want %v", grandparent.ID, ids[2])
	}

	// Get ancestor beyond chain
	_, err = traverser.GetAncestor(ids[0], 10)
	if err == nil {
		t.Error("GetAncestor beyond chain should return error")
	}
}

func TestGetParent(t *testing.T) {
	snapshots, ids := createLinearChain(3)
	loader := &mockLoader{snapshots: snapshots}
	traverser := history.NewTraverser(loader)

	parent, err := traverser.GetParent(ids[0])
	if err != nil {
		t.Fatalf("GetParent() error = %v", err)
	}

	if parent.ID != ids[1] {
		t.Errorf("GetParent() = %v, want %v", parent.ID, ids[1])
	}
}

func TestFindCommonAncestor_SameSnapshot(t *testing.T) {
	id := snapshot.NewID()
	snap := &snapshot.Snapshot{
		ID:   id,
		Data: map[string]any{},
	}

	loader := &mockLoader{
		snapshots: map[snapshot.ID]*snapshot.Snapshot{
			id: snap,
		},
	}
	traverser := history.NewTraverser(loader)

	common, err := traverser.FindCommonAncestor(id, id)
	if err != nil {
		t.Fatalf("FindCommonAncestor() error = %v", err)
	}

	if common.ID != id {
		t.Errorf("common ancestor = %v, want %v", common.ID, id)
	}
}

func TestFindCommonAncestor_LinearChain(t *testing.T) {
	snapshots, ids := createLinearChain(5)
	loader := &mockLoader{snapshots: snapshots}
	traverser := history.NewTraverser(loader)

	// ids[0] and ids[2] share common ancestor ids[2]
	common, err := traverser.FindCommonAncestor(ids[0], ids[2])
	if err != nil {
		t.Fatalf("FindCommonAncestor() error = %v", err)
	}

	if common.ID != ids[2] {
		t.Errorf("common ancestor = %v, want %v", common.ID, ids[2])
	}
}

func TestGetSnapshotsBetween(t *testing.T) {
	snapshots, ids := createLinearChain(5)
	loader := &mockLoader{snapshots: snapshots}
	traverser := history.NewTraverser(loader)

	// Get snapshots between ids[0] and ids[3]
	between, err := traverser.GetSnapshotsBetween(ids[0], ids[3])
	if err != nil {
		t.Fatalf("GetSnapshotsBetween() error = %v", err)
	}

	if len(between) != 4 {
		t.Errorf("len(between) = %d, want 4", len(between))
	}

	// Verify ordering
	for i := 0; i < len(between); i++ {
		if between[i].ID != ids[i] {
			t.Errorf("between[%d].ID = %v, want %v", i, between[i].ID, ids[i])
		}
	}
}

func TestGetSnapshotsBetween_SameSnapshot(t *testing.T) {
	snapshots, ids := createLinearChain(3)
	loader := &mockLoader{snapshots: snapshots}
	traverser := history.NewTraverser(loader)

	between, err := traverser.GetSnapshotsBetween(ids[0], ids[0])
	if err != nil {
		t.Fatalf("GetSnapshotsBetween() error = %v", err)
	}

	if len(between) != 1 {
		t.Errorf("len(between) = %d, want 1", len(between))
	}

	if between[0].ID != ids[0] {
		t.Errorf("between[0].ID = %v, want %v", between[0].ID, ids[0])
	}
}

func TestCountGenerations(t *testing.T) {
	snapshots, ids := createLinearChain(5)
	loader := &mockLoader{snapshots: snapshots}
	traverser := history.NewTraverser(loader)

	// From ids[0] to ids[3] is 3 generations
	count, err := traverser.CountGenerations(ids[0], ids[3])
	if err != nil {
		t.Fatalf("CountGenerations() error = %v", err)
	}

	if count != 3 {
		t.Errorf("CountGenerations() = %d, want 3", count)
	}

	// From ids[0] to ids[0] is 0 generations
	count, err = traverser.CountGenerations(ids[0], ids[0])
	if err != nil {
		t.Fatalf("CountGenerations() error = %v", err)
	}

	if count != 0 {
		t.Errorf("CountGenerations() = %d, want 0", count)
	}
}

func TestHistoryChain_Helpers(t *testing.T) {
	snapshots, ids := createLinearChain(5)
	loader := &mockLoader{snapshots: snapshots}
	traverser := history.NewTraverser(loader)

	chain, err := traverser.Traverse(analysis.HistoryQuery{
		StartID:      ids[0],
		IncludeStart: true,
	})
	if err != nil {
		t.Fatalf("Traverse() error = %v", err)
	}

	// Test Newest
	newest := chain.Newest()
	if newest.ID != ids[0] {
		t.Errorf("Newest() = %v, want %v", newest.ID, ids[0])
	}

	// Test Oldest
	oldest := chain.Oldest()
	if oldest.ID != ids[4] {
		t.Errorf("Oldest() = %v, want %v", oldest.ID, ids[4])
	}

	// Test IsEmpty
	if chain.IsEmpty() {
		t.Error("IsEmpty() = true, want false")
	}

	// Test empty chain
	emptyChain := &analysis.HistoryChain{}
	if !emptyChain.IsEmpty() {
		t.Error("empty chain IsEmpty() = false, want true")
	}
	if emptyChain.Newest() != nil {
		t.Error("empty chain Newest() should return nil")
	}
	if emptyChain.Oldest() != nil {
		t.Error("empty chain Oldest() should return nil")
	}
}

// Test cycle detection (defensive).
func TestTraverse_CycleDetection(t *testing.T) {
	// Create two snapshots that reference each other (invalid but defensive)
	id1 := snapshot.NewID()
	id2 := snapshot.NewID()

	snap1 := &snapshot.Snapshot{
		ID:       id1,
		ParentID: id2, // points to snap2
	}
	snap2 := &snapshot.Snapshot{
		ID:       id2,
		ParentID: id1, // points back to snap1 - CYCLE!
	}

	loader := &mockLoader{
		snapshots: map[snapshot.ID]*snapshot.Snapshot{
			id1: snap1,
			id2: snap2,
		},
	}
	traverser := history.NewTraverser(loader)

	// Should not infinite loop - cycle detection should stop it
	chain, err := traverser.Traverse(analysis.HistoryQuery{
		StartID:      id1,
		IncludeStart: true,
	})

	if err != nil {
		t.Fatalf("Traverse() error = %v", err)
	}

	// Should have stopped at the cycle
	if chain.Depth > 2 {
		t.Errorf("Depth = %d, cycle detection failed", chain.Depth)
	}
}
