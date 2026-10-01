// Package repairmap provides traversal queries over a dependency graph.
package repairmap

import (
	"fmt"

	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/graph"
)

// RepairMap wraps a Store and exposes dependency traversal queries.
type RepairMap struct {
	store graph.Store
}

// New creates a new RepairMap backed by the given Store.
func New(store graph.Store) (*RepairMap, error) {
	if store == nil {
		return nil, fmt.Errorf("repairmap: store must be non-nil")
	}
	return &RepairMap{store: store}, nil
}

// IsQueryable reports whether the graph has at least one entity.
func (rm *RepairMap) IsQueryable() bool {
	return rm.store.EntityCount() > 0
}

// EntityCount returns the number of entities in the graph.
func (rm *RepairMap) EntityCount() int {
	return rm.store.EntityCount()
}

// RelationCount returns the number of relations in the graph.
func (rm *RepairMap) RelationCount() int {
	return rm.store.RelationCount()
}

// AddEntity adds or updates an entity in the underlying graph.
func (rm *RepairMap) AddEntity(e *entity.Entity) error {
	return rm.store.AddEntity(e)
}

// AddRelation adds a relation to the underlying graph.
// Both endpoint entities must already exist.
func (rm *RepairMap) AddRelation(r *relation.Relation) error {
	return rm.store.AddRelation(r)
}

// GetEntity retrieves an entity by ID.
func (rm *RepairMap) GetEntity(id entity.ID) (*entity.Entity, error) {
	return rm.store.GetEntity(id)
}

// Store returns the underlying Store.
func (rm *RepairMap) Store() graph.Store {
	return rm.store
}

// ListEntities returns all entities matching the filter.
func (rm *RepairMap) ListEntities(filter graph.Filter) ([]*entity.Entity, error) {
	return rm.store.ListEntities(filter)
}

// ListRelations returns all relations matching the filter.
func (rm *RepairMap) ListRelations(filter graph.Filter) ([]*relation.Relation, error) {
	return rm.store.ListRelations(filter)
}

// Traversal queries

// DirectDependencies returns all entities that id directly depends on
// i.e., all entities e such that a relation (id → e) of kind depends_on or
// imports exists. Pass an empty kind to return all outgoing relation targets.
func (rm *RepairMap) DirectDependencies(id entity.ID, kind relation.Kind) ([]*entity.Entity, error) {
	rels, err := rm.store.ListRelations(graph.Filter{
		FromID:       id,
		RelationKind: kind,
	})
	if err != nil {
		return nil, fmt.Errorf("repairmap: DirectDependencies(%s): %w", id, err)
	}

	return rm.resolveTargets(rels)
}

// DirectDependents returns all entities that directly depend on id
// i.e., all entities e such that a relation (e → id) exists.
func (rm *RepairMap) DirectDependents(id entity.ID, kind relation.Kind) ([]*entity.Entity, error) {
	rels, err := rm.store.ListRelations(graph.Filter{
		ToID:         id,
		RelationKind: kind,
	})
	if err != nil {
		return nil, fmt.Errorf("repairmap: DirectDependents(%s): %w", id, err)
	}

	return rm.resolveSources(rels)
}

// Reachable returns all entities reachable from id by following relations of
// the given kind transitively. The starting entity is not included in the result.
// A depth limit prevents runaway traversal on cyclic graphs.
func (rm *RepairMap) Reachable(id entity.ID, kind relation.Kind, maxDepth int) ([]*entity.Entity, error) {
	if maxDepth <= 0 {
		maxDepth = 50 // defensive default
	}

	visited := make(map[entity.ID]bool)
	result := make([]*entity.Entity, 0)

	queue := []entity.ID{id}
	depth := 0

	for len(queue) > 0 && depth < maxDepth {
		next := make([]entity.ID, 0)
		for _, current := range queue {
			if visited[current] {
				continue
			}
			visited[current] = true

			rels, err := rm.store.ListRelations(graph.Filter{
				FromID:       current,
				RelationKind: kind,
			})
			if err != nil {
				return nil, fmt.Errorf("repairmap: Reachable traversal from %s: %w", current, err)
			}

			for _, r := range rels {
				if visited[r.ToID] {
					continue
				}
				e, err := rm.store.GetEntity(r.ToID)
				if err != nil {
					continue // entity may have been deleted; skip gracefully
				}
				if r.ToID != id { // exclude starting entity
					result = append(result, e)
				}
				next = append(next, r.ToID)
			}
		}
		queue = next
		depth++
	}

	return deduplicateEntities(result), nil
}

// ImpactSet returns all entities potentially affected by a change in id.
// It traverses reverse (incoming) edges to find transitive dependents.
func (rm *RepairMap) ImpactSet(id entity.ID, maxDepth int) ([]*entity.Entity, error) {
	if maxDepth <= 0 {
		maxDepth = 50
	}

	visited := make(map[entity.ID]bool)
	result := make([]*entity.Entity, 0)

	queue := []entity.ID{id}
	depth := 0

	for len(queue) > 0 && depth < maxDepth {
		next := make([]entity.ID, 0)
		for _, current := range queue {
			if visited[current] {
				continue
			}
			visited[current] = true

			// Follow all incoming relations (things that depend on current).
			rels, err := rm.store.ListRelations(graph.Filter{ToID: current})
			if err != nil {
				return nil, fmt.Errorf("repairmap: ImpactSet traversal from %s: %w", current, err)
			}

			for _, r := range rels {
				if visited[r.FromID] {
					continue
				}
				e, err := rm.store.GetEntity(r.FromID)
				if err != nil {
					continue
				}
				if r.FromID != id {
					result = append(result, e)
				}
				next = append(next, r.FromID)
			}
		}
		queue = next
		depth++
	}

	return deduplicateEntities(result), nil
}

// ShortestPath returns the shortest directed path of entity IDs from `from` to
// `to`, inclusive, using BFS. Returns nil if no path exists.
// kind constrains which relation types may be traversed; empty means any.
func (rm *RepairMap) ShortestPath(from, to entity.ID, kind relation.Kind) ([]entity.ID, error) {
	if from == to {
		return []entity.ID{from}, nil
	}

	// BFS: track parent for path reconstruction.
	parent := map[entity.ID]entity.ID{}
	visited := map[entity.ID]bool{from: true}
	queue := []entity.ID{from}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		rels, err := rm.store.ListRelations(graph.Filter{
			FromID:       current,
			RelationKind: kind,
		})
		if err != nil {
			return nil, fmt.Errorf("repairmap: ShortestPath traversal: %w", err)
		}

		for _, r := range rels {
			if visited[r.ToID] {
				continue
			}
			parent[r.ToID] = current
			if r.ToID == to {
				return reconstructPath(parent, from, to), nil
			}
			visited[r.ToID] = true
			queue = append(queue, r.ToID)
		}
	}

	return nil, nil // no path found
}

// AllRelationsFor returns all relations in which id participates, either as
// source (FromID) or target (ToID).
func (rm *RepairMap) AllRelationsFor(id entity.ID) ([]*relation.Relation, error) {
	outgoing, err := rm.store.ListRelations(graph.Filter{FromID: id})
	if err != nil {
		return nil, fmt.Errorf("repairmap: AllRelationsFor outgoing(%s): %w", id, err)
	}
	incoming, err := rm.store.ListRelations(graph.Filter{ToID: id})
	if err != nil {
		return nil, fmt.Errorf("repairmap: AllRelationsFor incoming(%s): %w", id, err)
	}

	// Merge and deduplicate.
	seen := make(map[relation.ID]bool)
	result := make([]*relation.Relation, 0, len(outgoing)+len(incoming))
	for _, r := range append(outgoing, incoming...) {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		result = append(result, r)
	}
	return result, nil
}

func (rm *RepairMap) resolveTargets(rels []*relation.Relation) ([]*entity.Entity, error) {
	result := make([]*entity.Entity, 0, len(rels))
	for _, r := range rels {
		e, err := rm.store.GetEntity(r.ToID)
		if err != nil {
			continue // entity may have been deleted; skip gracefully
		}
		result = append(result, e)
	}
	return result, nil
}

func (rm *RepairMap) resolveSources(rels []*relation.Relation) ([]*entity.Entity, error) {
	result := make([]*entity.Entity, 0, len(rels))
	for _, r := range rels {
		e, err := rm.store.GetEntity(r.FromID)
		if err != nil {
			continue
		}
		result = append(result, e)
	}
	return result, nil
}

func deduplicateEntities(entities []*entity.Entity) []*entity.Entity {
	seen := make(map[entity.ID]bool)
	result := make([]*entity.Entity, 0, len(entities))
	for _, e := range entities {
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		result = append(result, e)
	}
	return result
}

func reconstructPath(parent map[entity.ID]entity.ID, from, to entity.ID) []entity.ID {
	path := []entity.ID{to}
	current := to
	for current != from {
		current = parent[current]
		path = append([]entity.ID{current}, path...)
	}
	return path
}
