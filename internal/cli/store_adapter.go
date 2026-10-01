package cli

import (
	"context"

	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

// storeLoader adapts *store.FileStore to satisfy analysis.SnapshotLoader.
// The analysis SnapshotLoader interface has context-free signatures; this
// adapter bridges by supplying a background context.
type storeLoader struct {
	s *store.FileStore
}

func (sl *storeLoader) Load(id snapshot.ID) (*snapshot.Snapshot, error) {
	return sl.s.Load(context.Background(), id)
}

func (sl *storeLoader) List(_ interface{}) ([]*snapshot.Snapshot, error) {
	return sl.s.List(context.Background(), store.Filter{})
}
