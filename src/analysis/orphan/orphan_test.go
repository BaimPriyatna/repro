package orphan_test

import (
	"testing"

	"github.com/BaimPriyatna/repro/src/analysis/orphan"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
	"github.com/BaimPriyatna/repro/src/graph/store"
)

func TestOrphan_New_Validation(t *testing.T) {
	_, err := orphan.New(nil)
	if err == nil {
		t.Error("expected error for nil repair map, got nil")
	}
}

func TestOrphan_Analyze(t *testing.T) {
	s := store.NewMemStore()

	// Entities
	// 1. owner-team: a team entity
	// 2. svc-owned: service owned by owner-team
	// 3. svc-unowned: service with dependencies, but no owner
	// 4. pkg-orphan: package with 0 incoming relations and no owner
	// 5. file-isolated: completely isolated entity (0 relations)
	// 6. root-app: root application (metadata root: true)
	entities := []*entity.Entity{
		{ID: "owner-team", Kind: entity.KindModule, Name: "Platform Team"},
		{ID: "svc-owned", Kind: entity.KindService, Name: "payment-service"},
		{ID: "svc-unowned", Kind: entity.KindService, Name: "reporting-service"},
		{ID: "pkg-orphan", Kind: entity.KindPackage, Name: "leftover-lib"},
		{ID: "file-isolated", Kind: entity.KindFile, Name: "temp.bak"},
		{
			ID:         "root-app",
			Kind:       entity.KindService,
			Name:       "gateway",
			Attributes: map[string]any{"root": "true"},
		},
	}

	for _, e := range entities {
		if err := s.AddEntity(e); err != nil {
			t.Fatalf("AddEntity failed: %v", err)
		}
	}

	// Relations
	// svc-owned owned_by owner-team
	// root-app depends_on svc-owned
	// root-app depends_on svc-unowned (svc-unowned has incoming, but no owner)
	// pkg-orphan depends_on file-isolated? no, pkg-orphan has outgoing to nothing or isolated
	relations := []*relation.Relation{
		{ID: "r1", Kind: relation.KindOwnedBy, FromID: "svc-owned", ToID: "owner-team"},
		{ID: "r2", Kind: relation.KindDependsOn, FromID: "root-app", ToID: "svc-owned"},
		{ID: "r3", Kind: relation.KindDependsOn, FromID: "root-app", ToID: "svc-unowned"},
		{ID: "r4", Kind: relation.KindDependsOn, FromID: "pkg-orphan", ToID: "svc-owned"},
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

	analyzer, err := orphan.New(rm)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if analyzer.Name() != orphan.AnalyzerName {
		t.Errorf("Name() = %v, want %v", analyzer.Name(), orphan.AnalyzerName)
	}

	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.Status != coreanalysis.StatusFindings {
		t.Errorf("Status = %v, want %v", result.Status, coreanalysis.StatusFindings)
	}

	findingsBySubject := make(map[entity.ID]coreanalysis.Finding)
	for _, f := range result.Findings {
		if !f.HasEvidence() {
			t.Errorf("finding %s has no evidence (missing evidence)", f.ID)
		}
		findingsBySubject[f.Subject] = f
	}

	// file-isolated should be orphan_disconnected
	if f, ok := findingsBySubject["file-isolated"]; !ok {
		t.Errorf("expected finding for file-isolated")
	} else if f.Type != orphan.FindingTypeDisconnected {
		t.Errorf("finding type = %v, want %v", f.Type, orphan.FindingTypeDisconnected)
	}

	// pkg-orphan should be orphan_unreferenced
	if f, ok := findingsBySubject["pkg-orphan"]; !ok {
		t.Errorf("expected finding for pkg-orphan")
	} else if f.Type != orphan.FindingTypeUnreferenced {
		t.Errorf("finding type = %v, want %v", f.Type, orphan.FindingTypeUnreferenced)
	}

	// svc-unowned should be orphan_unowned
	if f, ok := findingsBySubject["svc-unowned"]; !ok {
		t.Errorf("expected finding for svc-unowned")
	} else if f.Type != orphan.FindingTypeUnowned {
		t.Errorf("finding type = %v, want %v", f.Type, orphan.FindingTypeUnowned)
	}

	// svc-owned and root-app should NOT be in findings
	if _, ok := findingsBySubject["svc-owned"]; ok {
		t.Errorf("svc-owned should not be reported as orphan")
	}
	if _, ok := findingsBySubject["root-app"]; ok {
		t.Errorf("root-app should not be reported as orphan")
	}
}
