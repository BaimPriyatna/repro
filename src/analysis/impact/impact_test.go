package impact_test

import (
	"testing"

	"github.com/BaimPriyatna/repro/src/analysis/impact"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
	"github.com/BaimPriyatna/repro/src/graph/store"
)

func setupTestGraph(t *testing.T) *repairmap.RepairMap {
	t.Helper()
	s := store.NewMemStore()

	// Entities
	// pkg-a (base library)
	// pkg-b (service that depends on pkg-a)
	// pkg-c (web app that depends on pkg-b)
	// pkg-d (isolated package)
	entities := []*entity.Entity{
		{ID: "pkg-a", Kind: entity.KindPackage, Name: "shared-lib"},
		{ID: "pkg-b", Kind: entity.KindService, Name: "auth-service"},
		{ID: "pkg-c", Kind: entity.KindModule, Name: "web-client"},
		{ID: "pkg-d", Kind: entity.KindPackage, Name: "unused-helper"},
	}
	for _, e := range entities {
		if err := s.AddEntity(e); err != nil {
			t.Fatalf("AddEntity failed: %v", err)
		}
	}

	// Relations
	// pkg-b depends_on pkg-a
	// pkg-c depends_on pkg-b
	relations := []*relation.Relation{
		{ID: "rel-1", Kind: relation.KindDependsOn, FromID: "pkg-b", ToID: "pkg-a"},
		{ID: "rel-2", Kind: relation.KindDependsOn, FromID: "pkg-c", ToID: "pkg-b"},
	}
	for _, r := range relations {
		if err := s.AddRelation(r); err != nil {
			t.Fatalf("AddRelation failed: %v", err)
		}
	}

	rm, err := repairmap.New(s)
	if err != nil {
		t.Fatalf("repairmap.New failed: %v", err)
	}
	return rm
}

func TestImpact_New_Validation(t *testing.T) {
	rm := setupTestGraph(t)

	// nil repair map
	_, err := impact.New(nil, []entity.ID{"pkg-a"})
	if err == nil {
		t.Error("expected error for nil repair map, got nil")
	}

	// empty changed entities
	_, err = impact.New(rm, nil)
	if err == nil {
		t.Error("expected error for empty changed entities, got nil")
	}
}

func TestImpact_Analyze_DirectAndTransitive(t *testing.T) {
	rm := setupTestGraph(t)

	analyzer, err := impact.New(rm, []entity.ID{"pkg-a"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if analyzer.Name() != impact.AnalyzerName {
		t.Errorf("Name() = %v, want %v", analyzer.Name(), impact.AnalyzerName)
	}
	if analyzer.Version() != impact.AnalyzerVersion {
		t.Errorf("Version() = %v, want %v", analyzer.Version(), impact.AnalyzerVersion)
	}

	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.Status != coreanalysis.StatusFindings {
		t.Errorf("Status = %v, want %v", result.Status, coreanalysis.StatusFindings)
	}

	// Should impact pkg-b and pkg-c, but NOT pkg-d or pkg-a itself
	if len(result.Findings) != 2 {
		t.Fatalf("len(Findings) = %d, want 2", len(result.Findings))
	}

	findingsBySubject := make(map[entity.ID]coreanalysis.Finding)
	for _, f := range result.Findings {
		if !f.HasEvidence() {
			t.Errorf("finding %s has no evidence (missing evidence)", f.ID)
		}
		if f.Type != impact.FindingTypePotentiallyAffected {
			t.Errorf("finding type = %v, want %v", f.Type, impact.FindingTypePotentiallyAffected)
		}
		findingsBySubject[f.Subject] = f
	}

	if _, ok := findingsBySubject["pkg-b"]; !ok {
		t.Errorf("expected finding for direct dependent pkg-b")
	}
	if _, ok := findingsBySubject["pkg-c"]; !ok {
		t.Errorf("expected finding for transitive dependent pkg-c")
	}
	if _, ok := findingsBySubject["pkg-d"]; ok {
		t.Errorf("unexpected finding for isolated pkg-d")
	}
}

func TestImpact_Analyze_LeafEntity(t *testing.T) {
	rm := setupTestGraph(t)

	// Changing top-level pkg-c has 0 dependents
	analyzer, err := impact.New(rm, []entity.ID{"pkg-c"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.Status != coreanalysis.StatusOK {
		t.Errorf("Status = %v, want %v", result.Status, coreanalysis.StatusOK)
	}
	if len(result.Findings) != 0 {
		t.Errorf("len(Findings) = %d, want 0", len(result.Findings))
	}
}

func TestImpact_Analyze_NonExistentEntity(t *testing.T) {
	rm := setupTestGraph(t)

	analyzer, err := impact.New(rm, []entity.ID{"non-existent"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.Status != coreanalysis.StatusOK {
		t.Errorf("Status = %v, want %v", result.Status, coreanalysis.StatusOK)
	}
	if len(result.Findings) != 0 {
		t.Errorf("len(Findings) = %d, want 0", len(result.Findings))
	}
}
