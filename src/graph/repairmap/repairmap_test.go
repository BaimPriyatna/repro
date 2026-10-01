package repairmap

import (
	"testing"

	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/graph"
	"github.com/BaimPriyatna/repro/src/graph/store"
)

// Helpers.
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

func mustAddEntity(t *testing.T, s graph.Store, e *entity.Entity) {
	t.Helper()
	if err := s.AddEntity(e); err != nil {
		t.Fatalf("AddEntity(%s): %v", e.ID, err)
	}
}

func mustAddRelation(t *testing.T, s graph.Store, r *relation.Relation) {
	t.Helper()
	if err := s.AddRelation(r); err != nil {
		t.Fatalf("AddRelation(%s): %v", r.ID, err)
	}
}

// buildGraph builds a RepairMap with a standard test graph:
//
// app --imports--> react
// app --imports--> lodash
// react --depends_on--> js-runtime
// lodash --depends_on--> js-runtime
// app --uses--> postgres.
func buildGraph(t *testing.T) *RepairMap {
	t.Helper()
	s := store.NewMemStore()

	mustAddEntity(t, s, makeEntity("app", "app", entity.KindModule))
	mustAddEntity(t, s, makeEntity("react", "react", entity.KindPackage))
	mustAddEntity(t, s, makeEntity("lodash", "lodash", entity.KindPackage))
	mustAddEntity(t, s, makeEntity("js-runtime", "node", entity.KindRuntime))
	mustAddEntity(t, s, makeEntity("postgres", "postgres", entity.KindService))

	mustAddRelation(t, s, makeRelation("r1", relation.KindImports, "app", "react"))
	mustAddRelation(t, s, makeRelation("r2", relation.KindImports, "app", "lodash"))
	mustAddRelation(t, s, makeRelation("r3", relation.KindDependsOn, "react", "js-runtime"))
	mustAddRelation(t, s, makeRelation("r4", relation.KindDependsOn, "lodash", "js-runtime"))
	mustAddRelation(t, s, makeRelation("r5", relation.KindUses, "app", "postgres"))

	rm, err := New(s)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return rm
}

// Constructor

func TestNew_NilStoreReturnsError(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil store, got nil")
	}
}

func TestNew_ValidStore(t *testing.T) {
	rm, err := New(store.NewMemStore())
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}
	if rm == nil {
		t.Fatal("expected non-nil RepairMap")
	}
}

// IsQueryable

func TestIsQueryable_EmptyGraphReturnsFalse(t *testing.T) {
	rm, _ := New(store.NewMemStore())
	if rm.IsQueryable() {
		t.Fatal("empty graph should not be queryable")
	}
}

func TestIsQueryable_WithEntitiesReturnsTrue(t *testing.T) {
	rm := buildGraph(t)
	if !rm.IsQueryable() {
		t.Fatal("populated graph should be queryable")
	}
}

// DirectDependencies

func TestDirectDependencies_ByKind(t *testing.T) {
	rm := buildGraph(t)
	deps, err := rm.DirectDependencies("app", relation.KindImports)
	if err != nil {
		t.Fatalf("DirectDependencies: %v", err)
	}
	if len(deps) != 2 {
		t.Errorf("expected 2 imports from app, got %d", len(deps))
	}
}

func TestDirectDependencies_AllKinds(t *testing.T) {
	rm := buildGraph(t)
	// Empty kind = all outgoing relations from app.
	deps, err := rm.DirectDependencies("app", "")
	if err != nil {
		t.Fatalf("DirectDependencies: %v", err)
	}
	// app --imports--> react, lodash; app --uses--> postgres = 3
	if len(deps) != 3 {
		t.Errorf("expected 3 dependencies from app (all kinds), got %d", len(deps))
	}
}

func TestDirectDependencies_NoOutgoing(t *testing.T) {
	rm := buildGraph(t)
	deps, err := rm.DirectDependencies("postgres", "")
	if err != nil {
		t.Fatalf("DirectDependencies: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 dependencies for postgres (leaf node), got %d", len(deps))
	}
}

// DirectDependents

func TestDirectDependents_JsRuntime(t *testing.T) {
	rm := buildGraph(t)
	dependents, err := rm.DirectDependents("js-runtime", relation.KindDependsOn)
	if err != nil {
		t.Fatalf("DirectDependents: %v", err)
	}
	// react and lodash both depend_on js-runtime.
	if len(dependents) != 2 {
		t.Errorf("expected 2 dependents of js-runtime, got %d", len(dependents))
	}
}

func TestDirectDependents_LeafNode_NoDependents(t *testing.T) {
	rm := buildGraph(t)
	dependents, err := rm.DirectDependents("app", "")
	if err != nil {
		t.Fatalf("DirectDependents: %v", err)
	}
	if len(dependents) != 0 {
		t.Errorf("expected 0 dependents for app (root), got %d", len(dependents))
	}
}

// Reachable

func TestReachable_FromApp_Imports(t *testing.T) {
	rm := buildGraph(t)
	reachable, err := rm.Reachable("app", relation.KindImports, 10)
	if err != nil {
		t.Fatalf("Reachable: %v", err)
	}
	// app --imports--> react, lodash. Neither react nor lodash has outgoing imports.
	if len(reachable) != 2 {
		t.Errorf("expected 2 reachable via imports from app, got %d", len(reachable))
	}
}

func TestReachable_FromApp_DependsOn_Transitive(t *testing.T) {
	rm := buildGraph(t)
	// app --imports--> react --depends_on--> js-runtime
	// Reachable via depends_on from app = none (app has no depends_on edges).
	// Reachable via depends_on from react = js-runtime.
	reachable, err := rm.Reachable("react", relation.KindDependsOn, 10)
	if err != nil {
		t.Fatalf("Reachable: %v", err)
	}
	if len(reachable) != 1 {
		t.Errorf("expected 1 reachable from react via depends_on, got %d", len(reachable))
	}
}

func TestReachable_CycleHandled(t *testing.T) {
	s := store.NewMemStore()
	mustAddEntity(t, s, makeEntity("a", "A", entity.KindModule))
	mustAddEntity(t, s, makeEntity("b", "B", entity.KindModule))
	mustAddEntity(t, s, makeEntity("c", "C", entity.KindModule))
	// a → b → c → a (cycle)
	mustAddRelation(t, s, makeRelation("r1", relation.KindDependsOn, "a", "b"))
	mustAddRelation(t, s, makeRelation("r2", relation.KindDependsOn, "b", "c"))
	mustAddRelation(t, s, makeRelation("r3", relation.KindDependsOn, "c", "a"))

	rm, _ := New(s)
	reachable, err := rm.Reachable("a", relation.KindDependsOn, 10)
	if err != nil {
		t.Fatalf("Reachable with cycle: %v", err)
	}
	// Should find b and c without looping forever.
	if len(reachable) != 2 {
		t.Errorf("expected 2 reachable in cycle (b, c), got %d", len(reachable))
	}
}

// ImpactSet

func TestImpactSet_JsRuntime_AffectsAll(t *testing.T) {
	rm := buildGraph(t)
	// js-runtime is depended on by react and lodash.
	// react and lodash are imported by app.
	// So a change in js-runtime potentially affects react, lodash, and app.
	impact, err := rm.ImpactSet("js-runtime", 10)
	if err != nil {
		t.Fatalf("ImpactSet: %v", err)
	}
	ids := entityIDs(impact)
	if !containsAll(ids, "react", "lodash", "app") {
		t.Errorf("expected react, lodash, app in impact set of js-runtime, got %v", ids)
	}
}

func TestImpactSet_LeafNode_EmptyImpact(t *testing.T) {
	rm := buildGraph(t)
	// app is the root; nothing depends on it.
	impact, err := rm.ImpactSet("app", 10)
	if err != nil {
		t.Fatalf("ImpactSet: %v", err)
	}
	if len(impact) != 0 {
		t.Errorf("expected 0 impact for root app, got %d: %v", len(impact), entityIDs(impact))
	}
}

func TestImpactSet_CycleHandled(t *testing.T) {
	s := store.NewMemStore()
	mustAddEntity(t, s, makeEntity("a", "A", entity.KindModule))
	mustAddEntity(t, s, makeEntity("b", "B", entity.KindModule))
	mustAddRelation(t, s, makeRelation("r1", relation.KindAffects, "a", "b"))
	mustAddRelation(t, s, makeRelation("r2", relation.KindAffects, "b", "a"))

	rm, _ := New(s)
	_, err := rm.ImpactSet("a", 10)
	if err != nil {
		t.Fatalf("ImpactSet with cycle: %v", err)
	}
}

// ShortestPath

func TestShortestPath_DirectEdge(t *testing.T) {
	rm := buildGraph(t)
	path, err := rm.ShortestPath("app", "react", relation.KindImports)
	if err != nil {
		t.Fatalf("ShortestPath: %v", err)
	}
	if len(path) != 2 {
		t.Errorf("expected path [app, react], got %v", path)
	}
	if path[0] != "app" || path[1] != "react" {
		t.Errorf("unexpected path: %v", path)
	}
}

func TestShortestPath_TransitivePath(t *testing.T) {
	rm := buildGraph(t)
	// app --imports--> react --depends_on--> js-runtime
	// No single-kind path from app to js-runtime (imports ≠ depends_on).
	// With empty kind (any relation type), path should exist.
	path, err := rm.ShortestPath("app", "js-runtime", "")
	if err != nil {
		t.Fatalf("ShortestPath: %v", err)
	}
	// app → react → js-runtime (or app → lodash → js-runtime)
	if len(path) != 3 {
		t.Errorf("expected path length 3, got %d: %v", len(path), path)
	}
	if path[0] != "app" || path[2] != "js-runtime" {
		t.Errorf("unexpected path endpoints: %v", path)
	}
}

func TestShortestPath_SameNode(t *testing.T) {
	rm := buildGraph(t)
	path, err := rm.ShortestPath("app", "app", "")
	if err != nil {
		t.Fatalf("ShortestPath: %v", err)
	}
	if len(path) != 1 || path[0] != "app" {
		t.Errorf("expected [app] for same-node path, got %v", path)
	}
}

func TestShortestPath_NoPath_ReturnsNil(t *testing.T) {
	rm := buildGraph(t)
	// postgres has no outgoing relations — no path from postgres to app.
	path, err := rm.ShortestPath("postgres", "app", "")
	if err != nil {
		t.Fatalf("ShortestPath: %v", err)
	}
	if path != nil {
		t.Errorf("expected nil path (no route), got %v", path)
	}
}

// AllRelationsFor

func TestAllRelationsFor_React(t *testing.T) {
	rm := buildGraph(t)
	// react: incoming r1 (app --imports--> react), outgoing r3 (react --depends_on--> js-runtime)
	rels, err := rm.AllRelationsFor("react")
	if err != nil {
		t.Fatalf("AllRelationsFor: %v", err)
	}
	if len(rels) != 2 {
		t.Errorf("expected 2 relations for react, got %d", len(rels))
	}
}

func TestAllRelationsFor_App_AllEdges(t *testing.T) {
	rm := buildGraph(t)
	// app has 3 outgoing relations (r1, r2, r5) and 0 incoming.
	rels, err := rm.AllRelationsFor("app")
	if err != nil {
		t.Fatalf("AllRelationsFor: %v", err)
	}
	if len(rels) != 3 {
		t.Errorf("expected 3 relations for app, got %d", len(rels))
	}
}

func TestAllRelationsFor_NoDuplicates(t *testing.T) {
	s := store.NewMemStore()
	mustAddEntity(t, s, makeEntity("a", "A", entity.KindModule))
	mustAddEntity(t, s, makeEntity("b", "B", entity.KindModule))
	// Single relation; appears in both outgoing(a) and incoming(b) queries internally.
	mustAddRelation(t, s, makeRelation("r1", relation.KindDependsOn, "a", "b"))

	rm, _ := New(s)
	relsA, _ := rm.AllRelationsFor("a")
	if len(relsA) != 1 {
		t.Errorf("expected 1 relation for a (no duplicates), got %d", len(relsA))
	}
}

// Internal helpers for tests

func entityIDs(entities []*entity.Entity) []entity.ID {
	ids := make([]entity.ID, len(entities))
	for i, e := range entities {
		ids[i] = e.ID
	}
	return ids
}

func containsAll(ids []entity.ID, targets ...string) bool {
	set := make(map[entity.ID]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	for _, t := range targets {
		if !set[entity.ID(t)] {
			return false
		}
	}
	return true
}
