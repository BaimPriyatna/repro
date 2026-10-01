// Package store provides the MemStore implementation of graph.Store.
//
// MemStore is a thread-safe, in-memory graph store suitable for unit tests,
// single-run analysis, and as the reference implementation against which
// future persistent stores (file-backed, SQLite, etc.) must be validated.
//
// It enforces the referential integrity rule: a relation cannot be added
// unless both its endpoint entities already exist in the store. Deleting
// an entity cascades to all relations that reference it (ErrEndpointMissing
// prevention at the delete side).
package store

import (
	"sync"

	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/graph"
)

// MemStore is a thread-safe, in-memory implementation of graph.Store.
type MemStore struct {
	mu        sync.RWMutex
	entities  map[entity.ID]*entity.Entity
	relations map[relation.ID]*relation.Relation
}

// NewMemStore creates and returns a new, empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{
		entities:  make(map[entity.ID]*entity.Entity),
		relations: make(map[relation.ID]*relation.Relation),
	}
}

// AddEntity adds or replaces an entity node in the graph.
func (m *MemStore) AddEntity(e *entity.Entity) error {
	if e == nil || e.ID == "" {
		return graph.ErrInvalidEntity
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entities[e.ID] = e
	return nil
}

// AddRelation adds a directed relation to the graph.
// Both endpoint entities must already exist.
func (m *MemStore) AddRelation(r *relation.Relation) error {
	if r == nil || r.ID == "" || r.Kind == "" || r.FromID == "" || r.ToID == "" {
		return graph.ErrInvalidRelation
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.relations[r.ID]; ok {
		return graph.ErrDuplicateRelation
	}
	if _, ok := m.entities[r.FromID]; !ok {
		return graph.ErrEndpointMissing
	}
	if _, ok := m.entities[r.ToID]; !ok {
		return graph.ErrEndpointMissing
	}
	m.relations[r.ID] = r
	return nil
}

// GetEntity retrieves an entity by ID.
func (m *MemStore) GetEntity(id entity.ID) (*entity.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entities[id]
	if !ok {
		return nil, graph.ErrEntityNotFound
	}
	return e, nil
}

// GetRelation retrieves a relation by ID.
func (m *MemStore) GetRelation(id relation.ID) (*relation.Relation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.relations[id]
	if !ok {
		return nil, graph.ErrRelationNotFound
	}
	return r, nil
}

// ListEntities returns all entities matching the filter.
func (m *MemStore) ListEntities(f graph.Filter) ([]*entity.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*entity.Entity, 0, len(m.entities))
	for _, e := range m.entities {
		if f.EntityKind != "" && e.Kind != f.EntityKind {
			continue
		}
		result = append(result, e)
	}
	return result, nil
}

// ListRelations returns all relations matching the filter.
func (m *MemStore) ListRelations(f graph.Filter) ([]*relation.Relation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*relation.Relation, 0, len(m.relations))
	for _, r := range m.relations {
		if f.FromID != "" && r.FromID != f.FromID {
			continue
		}
		if f.ToID != "" && r.ToID != f.ToID {
			continue
		}
		if f.RelationKind != "" && r.Kind != f.RelationKind {
			continue
		}
		result = append(result, r)
	}
	return result, nil
}

// DeleteEntity removes an entity and cascades to all relations referencing it.
func (m *MemStore) DeleteEntity(id entity.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.entities[id]; !ok {
		return graph.ErrEntityNotFound
	}
	delete(m.entities, id)

	// Cascade: remove all relations that reference this entity.
	for rid, r := range m.relations {
		if r.FromID == id || r.ToID == id {
			delete(m.relations, rid)
		}
	}
	return nil
}

// DeleteRelation removes a relation by ID.
func (m *MemStore) DeleteRelation(id relation.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.relations[id]; !ok {
		return graph.ErrRelationNotFound
	}
	delete(m.relations, id)
	return nil
}

// EntityCount returns the number of stored entities.
func (m *MemStore) EntityCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entities)
}

// RelationCount returns the number of stored relations.
func (m *MemStore) RelationCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.relations)
}
