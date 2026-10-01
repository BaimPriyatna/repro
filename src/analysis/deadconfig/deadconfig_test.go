package deadconfig_test

import (
	"testing"

	"github.com/BaimPriyatna/repro/src/analysis/deadconfig"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
	"github.com/BaimPriyatna/repro/src/graph/store"
)

func TestDeadConfig_New_Validation(t *testing.T) {
	_, err := deadconfig.New(nil)
	if err == nil {
		t.Error("expected error for nil repair map, got nil")
	}
}

func TestDeadConfig_Analyze(t *testing.T) {
	s := store.NewMemStore()

	// 1. cfg-used: used by active service
	// 2. cfg-definitely-unused: 0 references
	// 3. cfg-probably-unused: referenced only by orphan helper
	// 4. cfg-unknown: dynamic usage
	// 5. svc-main: active service (depended on by root)
	// 6. helper-dead: orphan helper (0 incoming)
	entities := []*entity.Entity{
		{ID: "cfg-used", Kind: entity.KindConfig, Name: "DB_PORT"},
		{ID: "cfg-definitely-unused", Kind: entity.KindConfig, Name: "LEGACY_FLAG"},
		{ID: "cfg-probably-unused", Kind: entity.KindConfig, Name: "OLD_API_KEY"},
		{
			ID:         "cfg-unknown",
			Kind:       entity.KindConfig,
			Name:       "DYNAMIC_PARAM",
			Attributes: map[string]any{"dynamic": "true"},
		},
		{ID: "svc-main", Kind: entity.KindService, Name: "api-server"},
		{ID: "helper-dead", Kind: entity.KindPackage, Name: "dead-script"},
		{ID: "gateway", Kind: entity.KindService, Name: "gateway"},
	}

	for _, e := range entities {
		if err := s.AddEntity(e); err != nil {
			t.Fatalf("AddEntity failed: %v", err)
		}
	}

	// Relations
	// gateway -> svc-main (svc-main is active)
	// svc-main configured_by cfg-used
	// helper-dead uses cfg-probably-unused (helper-dead has 0 incoming)
	relations := []*relation.Relation{
		{ID: "r1", Kind: relation.KindDependsOn, FromID: "gateway", ToID: "svc-main"},
		{ID: "r2", Kind: relation.KindConfiguredBy, FromID: "svc-main", ToID: "cfg-used"},
		{ID: "r3", Kind: relation.KindUses, FromID: "helper-dead", ToID: "cfg-probably-unused"},
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

	analyzer, err := deadconfig.New(rm)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if analyzer.Name() != deadconfig.AnalyzerName {
		t.Errorf("Name() = %v, want %v", analyzer.Name(), deadconfig.AnalyzerName)
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

	// cfg-used should NOT be in findings
	if _, ok := findingsBySubject["cfg-used"]; ok {
		t.Errorf("cfg-used should not be reported as dead config")
	}

	// cfg-definitely-unused
	if f, ok := findingsBySubject["cfg-definitely-unused"]; !ok {
		t.Errorf("expected finding for cfg-definitely-unused")
	} else if f.Type != deadconfig.FindingTypeDefinitelyUnused {
		t.Errorf("finding type = %v, want %v", f.Type, deadconfig.FindingTypeDefinitelyUnused)
	}

	// cfg-probably-unused
	if f, ok := findingsBySubject["cfg-probably-unused"]; !ok {
		t.Errorf("expected finding for cfg-probably-unused")
	} else if f.Type != deadconfig.FindingTypeProbablyUnused {
		t.Errorf("finding type = %v, want %v", f.Type, deadconfig.FindingTypeProbablyUnused)
	}

	// cfg-unknown
	if f, ok := findingsBySubject["cfg-unknown"]; !ok {
		t.Errorf("expected finding for cfg-unknown")
	} else if f.Type != deadconfig.FindingTypeUnknown {
		t.Errorf("finding type = %v, want %v", f.Type, deadconfig.FindingTypeUnknown)
	}
}
