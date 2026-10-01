package depspy

import (
	"testing"

	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/graph"
	"github.com/BaimPriyatna/repro/src/graph/store"
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

func newDepspy(t *testing.T, declared, observed graph.Store) *Depspy {
	t.Helper()
	d, err := New(declared, observed)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

func countByType(findings []coreanalysis.Finding, findingType string) int {
	n := 0
	for _, f := range findings {
		if f.Type == findingType {
			n++
		}
	}
	return n
}

// Constructor

func TestNew_NilDeclaredReturnsError(t *testing.T) {
	obs := store.NewMemStore()
	_, err := New(nil, obs)
	if err == nil {
		t.Fatal("expected error for nil declared, got nil")
	}
}

func TestNew_NilObservedReturnsError(t *testing.T) {
	decl := store.NewMemStore()
	_, err := New(decl, nil)
	if err == nil {
		t.Fatal("expected error for nil observed, got nil")
	}
}

func TestNew_ValidStoresSucceeds(t *testing.T) {
	_, err := New(store.NewMemStore(), store.NewMemStore())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Analyze — no findings (clean state)

func TestAnalyze_BothEmpty_NoFindings(t *testing.T) {
	d := newDepspy(t, store.NewMemStore(), store.NewMemStore())
	result, err := d.Analyze()
	if err != nil {
		t.Fatalf("Analyze: unexpected error: %v", err)
	}
	if result.Status != coreanalysis.StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
}

func TestAnalyze_DeclaredAndObservedIdentical_NoFindings(t *testing.T) {
	decl := store.NewMemStore()
	obs := store.NewMemStore()

	// Add same entities and relations to both stores.
	for _, s := range []graph.Store{decl, obs} {
		mustAddEntity(t, s, makeEntity("react", "react", entity.KindPackage))
		mustAddEntity(t, s, makeEntity("app", "app", entity.KindModule))
		mustAddRelation(t, s, makeRelation("r1", relation.KindImports, "app", "react"))
	}

	d := newDepspy(t, decl, obs)
	result, err := d.Analyze()
	if err != nil {
		t.Fatalf("Analyze: unexpected error: %v", err)
	}
	if result.Status != coreanalysis.StatusOK {
		t.Errorf("expected StatusOK, got %s; findings: %+v", result.Status, result.Findings)
	}
}

// Hidden dependencies

func TestAnalyze_HiddenDependency_ObservedNotDeclared(t *testing.T) {
	decl := store.NewMemStore()
	obs := store.NewMemStore()

	// Declared: app → react only.
	mustAddEntity(t, decl, makeEntity("app", "app", entity.KindModule))
	mustAddEntity(t, decl, makeEntity("react", "react", entity.KindPackage))
	mustAddRelation(t, decl, makeRelation("r1", relation.KindImports, "app", "react"))

	// Observed: app also uses lodash (not declared).
	mustAddEntity(t, obs, makeEntity("app", "app", entity.KindModule))
	mustAddEntity(t, obs, makeEntity("react", "react", entity.KindPackage))
	mustAddEntity(t, obs, makeEntity("lodash", "lodash", entity.KindPackage))
	mustAddRelation(t, obs, makeRelation("r1", relation.KindImports, "app", "react"))
	mustAddRelation(t, obs, makeRelation("r2", relation.KindImports, "app", "lodash"))

	d := newDepspy(t, decl, obs)
	result, err := d.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	hidden := countByType(result.Findings, "hidden_dependency")
	if hidden != 1 {
		t.Errorf("expected 1 hidden_dependency finding, got %d", hidden)
	}
	if result.Status != coreanalysis.StatusFindings {
		t.Errorf("expected StatusFindings, got %s", result.Status)
	}
}

func TestAnalyze_MultipleHiddenDependencies(t *testing.T) {
	decl := store.NewMemStore()
	obs := store.NewMemStore()

	mustAddEntity(t, decl, makeEntity("app", "app", entity.KindModule))

	mustAddEntity(t, obs, makeEntity("app", "app", entity.KindModule))
	mustAddEntity(t, obs, makeEntity("lodash", "lodash", entity.KindPackage))
	mustAddEntity(t, obs, makeEntity("axios", "axios", entity.KindPackage))
	mustAddRelation(t, obs, makeRelation("r1", relation.KindImports, "app", "lodash"))
	mustAddRelation(t, obs, makeRelation("r2", relation.KindImports, "app", "axios"))

	d := newDepspy(t, decl, obs)
	result, err := d.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	hidden := countByType(result.Findings, "hidden_dependency")
	if hidden != 2 {
		t.Errorf("expected 2 hidden_dependency findings, got %d", hidden)
	}
}

// Undeclared relations

func TestAnalyze_UndeclaredRelation(t *testing.T) {
	decl := store.NewMemStore()
	obs := store.NewMemStore()

	// Both stores know about the same entities.
	for _, s := range []graph.Store{decl, obs} {
		mustAddEntity(t, s, makeEntity("app", "app", entity.KindModule))
		mustAddEntity(t, s, makeEntity("db", "db", entity.KindService))
	}

	// Declared: no explicit relation.
	// Observed: app uses db.
	mustAddRelation(t, obs, makeRelation("r-app-db", relation.KindUses, "app", "db"))

	d := newDepspy(t, decl, obs)
	result, err := d.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	undeclared := countByType(result.Findings, "undeclared_relation")
	if undeclared != 1 {
		t.Errorf("expected 1 undeclared_relation finding, got %d", undeclared)
	}
}

func TestAnalyze_SemanticEquivalentRelationNotFlagged(t *testing.T) {
	decl := store.NewMemStore()
	obs := store.NewMemStore()

	for _, s := range []graph.Store{decl, obs} {
		mustAddEntity(t, s, makeEntity("app", "app", entity.KindModule))
		mustAddEntity(t, s, makeEntity("db", "db", entity.KindService))
	}

	// Declared with ID "r-v1", observed with ID "r-v2" — same semantic meaning.
	mustAddRelation(t, decl, makeRelation("r-v1", relation.KindUses, "app", "db"))
	mustAddRelation(t, obs, makeRelation("r-v2", relation.KindUses, "app", "db"))

	d := newDepspy(t, decl, obs)
	result, err := d.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	undeclared := countByType(result.Findings, "undeclared_relation")
	if undeclared != 0 {
		t.Errorf("expected 0 undeclared_relation (semantic equiv declared), got %d", undeclared)
	}
}

// Unused declarations

func TestAnalyze_UnusedDeclaration(t *testing.T) {
	decl := store.NewMemStore()
	obs := store.NewMemStore()

	// Declared: both react and lodash declared.
	mustAddEntity(t, decl, makeEntity("app", "app", entity.KindModule))
	mustAddEntity(t, decl, makeEntity("react", "react", entity.KindPackage))
	mustAddEntity(t, decl, makeEntity("lodash", "lodash", entity.KindPackage))
	mustAddRelation(t, decl, makeRelation("r1", relation.KindImports, "app", "react"))

	// Observed: only react is actually used. Lodash is absent from observed.
	mustAddEntity(t, obs, makeEntity("app", "app", entity.KindModule))
	mustAddEntity(t, obs, makeEntity("react", "react", entity.KindPackage))
	mustAddRelation(t, obs, makeRelation("r1", relation.KindImports, "app", "react"))

	d := newDepspy(t, decl, obs)
	result, err := d.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	unused := countByType(result.Findings, "unused_declaration")
	if unused != 1 {
		t.Errorf("expected 1 unused_declaration finding, got %d", unused)
	}
}

// Evidence requirement

func TestAnalyze_AllFindingsHaveEvidence(t *testing.T) {
	decl := store.NewMemStore()
	obs := store.NewMemStore()

	mustAddEntity(t, decl, makeEntity("app", "app", entity.KindModule))
	mustAddEntity(t, decl, makeEntity("declared-only", "declared-only", entity.KindPackage))

	mustAddEntity(t, obs, makeEntity("app", "app", entity.KindModule))
	mustAddEntity(t, obs, makeEntity("hidden-pkg", "hidden-pkg", entity.KindPackage))
	mustAddRelation(t, obs, makeRelation("r-obs", relation.KindImports, "app", "hidden-pkg"))

	d := newDepspy(t, decl, obs)
	result, err := d.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	for _, f := range result.Findings {
		if !f.HasEvidence() {
			t.Errorf("finding %q has no evidence (missing evidence)", f.ID)
		}
	}
}

// AnalysisResult contract

func TestAnalyze_ResultContractFields(t *testing.T) {
	d := newDepspy(t, store.NewMemStore(), store.NewMemStore())
	result, err := d.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.ID == "" {
		t.Error("result.ID must not be empty")
	}
	if result.Analyzer != AnalyzerName {
		t.Errorf("expected Analyzer=%q, got %q", AnalyzerName, result.Analyzer)
	}
	if result.Version != AnalyzerVersion {
		t.Errorf("expected Version=%q, got %q", AnalyzerVersion, result.Version)
	}
	if result.Timestamp.IsZero() {
		t.Error("result.Timestamp must not be zero")
	}
}
