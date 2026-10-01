// Package graph defines a directed, typed dependency graph of entities and relations.
package graph

import (
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
)

// Node is an Entity in the graph, annotated with its adjacency metadata.
// Storing entities as Nodes (rather than embedding the entity directly in edges)
// keeps the graph store indexable and avoids duplication when an entity
// participates in many relations.
type Node struct {
	// Entity is the core domain entity this node wraps.
	Entity *entity.Entity

	// OutDegree is the number of outgoing edges from this node (informational).
	OutDegree int

	// InDegree is the number of incoming edges to this node (informational).
	InDegree int
}

// ID returns the entity ID for this node.
func (n *Node) ID() entity.ID {
	return n.Entity.ID
}

// Edge is a directed Relation in the graph, with resolved endpoint metadata.
type Edge struct {
	// Relation is the core domain relation this edge wraps.
	Relation *relation.Relation

	// From is the source node (the dependent or user).
	From *Node

	// To is the target node (the dependency or resource).
	To *Node
}

// Filter defines parameters for querying the graph.
// All fields are optional; zero values mean "no constraint on this dimension".
type Filter struct {
	// FromID constrains results to edges originating from this entity.
	FromID entity.ID

	// ToID constrains results to edges pointing to this entity.
	ToID entity.ID

	// RelationKind constrains results to edges of this relation kind.
	// Empty string means any kind.
	RelationKind relation.Kind

	// EntityKind constrains node queries to entities of this kind.
	// Empty string means any kind.
	EntityKind entity.Kind
}

// Store is the persistence and traversal interface for the dependency graph.
// Implementations may be in-memory, file-backed, or database-backed.
//
// All writes are additive — existing nodes and relations are never silently
// modified. To update an entity or relation, callers must
// explicitly delete and re-add.
type Store interface {
	// AddEntity adds or replaces an entity node in the graph.
	// If an entity with the same ID already exists, it is overwritten.
	AddEntity(e *entity.Entity) error

	// AddRelation adds a directed relation (edge) to the graph.
	// Both endpoint entities must already exist in the graph (ErrEntityNotFound
	// otherwise). Duplicate relation IDs are rejected with ErrDuplicateRelation.
	AddRelation(r *relation.Relation) error

	// GetEntity retrieves an entity by ID. Returns ErrEntityNotFound if absent.
	GetEntity(id entity.ID) (*entity.Entity, error)

	// GetRelation retrieves a relation by ID. Returns ErrRelationNotFound if absent.
	GetRelation(id relation.ID) (*relation.Relation, error)

	// ListEntities returns all entities matching the filter.
	// An empty Filter returns all entities.
	ListEntities(f Filter) ([]*entity.Entity, error)

	// ListRelations returns all relations matching the filter.
	// An empty Filter returns all relations.
	ListRelations(f Filter) ([]*relation.Relation, error)

	// DeleteEntity removes an entity and all relations that reference it.
	// Returns ErrEntityNotFound if the entity does not exist.
	DeleteEntity(id entity.ID) error

	// DeleteRelation removes a relation by ID.
	// Returns ErrRelationNotFound if the relation does not exist.
	DeleteRelation(id relation.ID) error

	// EntityCount returns the total number of stored entities.
	EntityCount() int

	// RelationCount returns the total number of stored relations.
	RelationCount() int
}
