package graph

import "github.com/BaimPriyatna/repro/src/core/errors"

// Graph-specific errors.
var (
	// ErrEntityNotFound is returned when a requested entity does not exist in the graph.
	ErrEntityNotFound = errors.New(errors.CodeNotFound, "entity not found in graph")

	// ErrRelationNotFound is returned when a requested relation does not exist.
	ErrRelationNotFound = errors.New(errors.CodeNotFound, "relation not found in graph")

	// ErrDuplicateRelation is returned when adding a relation whose ID already exists.
	ErrDuplicateRelation = errors.New(errors.CodeInvalidInput, "relation with this ID already exists")

	// ErrEndpointMissing is returned when a relation references an entity that
	// does not yet exist in the graph.
	ErrEndpointMissing = errors.New(errors.CodeInvalidInput, "relation endpoint entity not found in graph")

	// ErrInvalidEntity is returned when a nil or structurally invalid entity is supplied.
	ErrInvalidEntity = errors.New(errors.CodeInvalidInput, "invalid entity: must have non-empty ID")

	// ErrInvalidRelation is returned when a nil or structurally invalid relation is supplied.
	ErrInvalidRelation = errors.New(errors.CodeInvalidInput, "invalid relation: must have non-empty ID, Kind, FromID, and ToID")
)
