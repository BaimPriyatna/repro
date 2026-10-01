package store

import (
	"testing"

	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/graph"
)

// Helpers

func makeEntity(id, name string, kind entity.Kind) *entity.Entity {
	return &entity.Entity{ID: entity.ID(id), Kind: kind, Name: name}
}

func makeRelation(id string, kind relation.Kind, from, to entity.ID) *relation.Relation {
	return &relation.Relation{
		ID:     relation.ID(id),
		Kind:   kind,
		FromID: from,
		ToID:   to,
	}
}

// AddEntity

func TestMemStore_AddEntity_Basic(t *testing.T) {
	s := NewMemStore()
	e := makeEntity("e1", "react", entity.KindPackage)
	if err := s.AddEntity(e); err != nil {
		t.Fatalf("AddEntity: unexpected error: %v", err)
	}
	if s.EntityCount() != 1 {
		t.Fatalf("expected 1 entity, got %d", s.EntityCount())
	}
}

func TestMemStore_AddEntity_NilReturnsError(t *testing.T) {
	s := NewMemStore()
	if err := s.AddEntity(nil); err == nil {
		t.Fatal("expected error for nil entity, got nil")
	}
}

func TestMemStore_AddEntity_EmptyIDReturnsError(t *testing.T) {
	s := NewMemStore()
	e := &entity.Entity{ID: "", Kind: entity.KindPackage, Name: "x"}
	if err := s.AddEntity(e); err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

func TestMemStore_AddEntity_OverwritesExisting(t *testing.T) {
	s := NewMemStore()
	e1 := makeEntity("e1", "original", entity.KindPackage)
	e2 := makeEntity("e1", "updated", entity.KindPackage)
	_ = s.AddEntity(e1)
	_ = s.AddEntity(e2)
	got, _ := s.GetEntity("e1")
	if got.Name != "updated" {
		t.Fatalf("expected overwrite to 'updated', got %q", got.Name)
	}
	if s.EntityCount() != 1 {
		t.Fatalf("expected 1 entity after overwrite, got %d", s.EntityCount())
	}
}

// AddRelation

func TestMemStore_AddRelation_Basic(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("a", "A", entity.KindPackage))
	_ = s.AddEntity(makeEntity("b", "B", entity.KindPackage))
	r := makeRelation("r1", relation.KindDependsOn, "a", "b")
	if err := s.AddRelation(r); err != nil {
		t.Fatalf("AddRelation: unexpected error: %v", err)
	}
	if s.RelationCount() != 1 {
		t.Fatalf("expected 1 relation, got %d", s.RelationCount())
	}
}

func TestMemStore_AddRelation_MissingFromEndpoint(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("b", "B", entity.KindPackage))
	r := makeRelation("r1", relation.KindDependsOn, "missing", "b")
	if err := s.AddRelation(r); err == nil {
		t.Fatal("expected ErrEndpointMissing, got nil")
	}
}

func TestMemStore_AddRelation_MissingToEndpoint(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("a", "A", entity.KindPackage))
	r := makeRelation("r1", relation.KindDependsOn, "a", "missing")
	if err := s.AddRelation(r); err == nil {
		t.Fatal("expected ErrEndpointMissing, got nil")
	}
}

func TestMemStore_AddRelation_DuplicateIDReturnsError(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("a", "A", entity.KindPackage))
	_ = s.AddEntity(makeEntity("b", "B", entity.KindPackage))
	r := makeRelation("r1", relation.KindDependsOn, "a", "b")
	_ = s.AddRelation(r)
	if err := s.AddRelation(r); err == nil {
		t.Fatal("expected ErrDuplicateRelation, got nil")
	}
}

func TestMemStore_AddRelation_NilReturnsError(t *testing.T) {
	s := NewMemStore()
	if err := s.AddRelation(nil); err == nil {
		t.Fatal("expected error for nil relation, got nil")
	}
}

// GetEntity / GetRelation

func TestMemStore_GetEntity_NotFound(t *testing.T) {
	s := NewMemStore()
	_, err := s.GetEntity("nonexistent")
	if err == nil {
		t.Fatal("expected ErrEntityNotFound, got nil")
	}
}

func TestMemStore_GetRelation_NotFound(t *testing.T) {
	s := NewMemStore()
	_, err := s.GetRelation("nonexistent")
	if err == nil {
		t.Fatal("expected ErrRelationNotFound, got nil")
	}
}

func TestMemStore_GetEntity_ReturnsCorrectEntity(t *testing.T) {
	s := NewMemStore()
	e := makeEntity("pkg-go", "go", entity.KindRuntime)
	_ = s.AddEntity(e)
	got, err := s.GetEntity("pkg-go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "go" {
		t.Errorf("expected name 'go', got %q", got.Name)
	}
}

// ListEntities

func TestMemStore_ListEntities_Empty(t *testing.T) {
	s := NewMemStore()
	entities, err := s.ListEntities(graph.Filter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 0 {
		t.Fatalf("expected 0 entities, got %d", len(entities))
	}
}

func TestMemStore_ListEntities_FilterByKind(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("p1", "react", entity.KindPackage))
	_ = s.AddEntity(makeEntity("p2", "vue", entity.KindPackage))
	_ = s.AddEntity(makeEntity("r1", "node", entity.KindRuntime))

	pkgs, err := s.ListEntities(graph.Filter{EntityKind: entity.KindPackage})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}
}

func TestMemStore_ListEntities_NoFilterReturnsAll(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("p1", "react", entity.KindPackage))
	_ = s.AddEntity(makeEntity("r1", "node", entity.KindRuntime))

	all, _ := s.ListEntities(graph.Filter{})
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
}

// ListRelations

func TestMemStore_ListRelations_FilterByFrom(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("a", "A", entity.KindPackage))
	_ = s.AddEntity(makeEntity("b", "B", entity.KindPackage))
	_ = s.AddEntity(makeEntity("c", "C", entity.KindPackage))
	_ = s.AddRelation(makeRelation("r1", relation.KindDependsOn, "a", "b"))
	_ = s.AddRelation(makeRelation("r2", relation.KindDependsOn, "a", "c"))
	_ = s.AddRelation(makeRelation("r3", relation.KindDependsOn, "b", "c"))

	fromA, _ := s.ListRelations(graph.Filter{FromID: "a"})
	if len(fromA) != 2 {
		t.Fatalf("expected 2 relations from 'a', got %d", len(fromA))
	}
}

func TestMemStore_ListRelations_FilterByTo(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("a", "A", entity.KindPackage))
	_ = s.AddEntity(makeEntity("b", "B", entity.KindPackage))
	_ = s.AddEntity(makeEntity("c", "C", entity.KindPackage))
	_ = s.AddRelation(makeRelation("r1", relation.KindDependsOn, "a", "c"))
	_ = s.AddRelation(makeRelation("r2", relation.KindDependsOn, "b", "c"))
	_ = s.AddRelation(makeRelation("r3", relation.KindDependsOn, "a", "b"))

	toC, _ := s.ListRelations(graph.Filter{ToID: "c"})
	if len(toC) != 2 {
		t.Fatalf("expected 2 relations to 'c', got %d", len(toC))
	}
}

func TestMemStore_ListRelations_FilterByKind(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("a", "A", entity.KindPackage))
	_ = s.AddEntity(makeEntity("b", "B", entity.KindPackage))
	_ = s.AddRelation(makeRelation("r1", relation.KindDependsOn, "a", "b"))
	_ = s.AddRelation(makeRelation("r2", relation.KindImports, "a", "b"))

	deps, _ := s.ListRelations(graph.Filter{RelationKind: relation.KindDependsOn})
	if len(deps) != 1 {
		t.Fatalf("expected 1 depends_on relation, got %d", len(deps))
	}
}

// DeleteEntity (cascade)

func TestMemStore_DeleteEntity_NotFound(t *testing.T) {
	s := NewMemStore()
	if err := s.DeleteEntity("nonexistent"); err == nil {
		t.Fatal("expected ErrEntityNotFound, got nil")
	}
}

func TestMemStore_DeleteEntity_CascadeDeletesRelations(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("a", "A", entity.KindPackage))
	_ = s.AddEntity(makeEntity("b", "B", entity.KindPackage))
	_ = s.AddEntity(makeEntity("c", "C", entity.KindPackage))
	_ = s.AddRelation(makeRelation("r1", relation.KindDependsOn, "a", "b"))
	_ = s.AddRelation(makeRelation("r2", relation.KindDependsOn, "c", "b"))
	_ = s.AddRelation(makeRelation("r3", relation.KindDependsOn, "a", "c"))

	// Delete 'b' — should cascade r1 and r2, but not r3.
	if err := s.DeleteEntity("b"); err != nil {
		t.Fatalf("DeleteEntity: unexpected error: %v", err)
	}
	if s.EntityCount() != 2 {
		t.Fatalf("expected 2 entities after delete, got %d", s.EntityCount())
	}
	if s.RelationCount() != 1 {
		t.Fatalf("expected 1 relation after cascade, got %d", s.RelationCount())
	}
}

// DeleteRelation

func TestMemStore_DeleteRelation_NotFound(t *testing.T) {
	s := NewMemStore()
	if err := s.DeleteRelation("nonexistent"); err == nil {
		t.Fatal("expected ErrRelationNotFound, got nil")
	}
}

func TestMemStore_DeleteRelation_RemovesRelation(t *testing.T) {
	s := NewMemStore()
	_ = s.AddEntity(makeEntity("a", "A", entity.KindPackage))
	_ = s.AddEntity(makeEntity("b", "B", entity.KindPackage))
	_ = s.AddRelation(makeRelation("r1", relation.KindDependsOn, "a", "b"))
	if err := s.DeleteRelation("r1"); err != nil {
		t.Fatalf("DeleteRelation: unexpected error: %v", err)
	}
	if s.RelationCount() != 0 {
		t.Fatalf("expected 0 relations, got %d", s.RelationCount())
	}
}

// Concurrency

func TestMemStore_ConcurrentWrites(t *testing.T) {
	s := NewMemStore()
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		id := entity.ID(string(rune('a'+i%26)) + string(rune('0'+i/26)))
		go func(eid entity.ID, name string) {
			_ = s.AddEntity(&entity.Entity{ID: eid, Kind: entity.KindPackage, Name: name})
			done <- struct{}{}
		}(id, string(id))
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	if s.EntityCount() == 0 {
		t.Fatal("expected entities after concurrent writes, got 0")
	}
}
