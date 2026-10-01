package ghostfile_test

import (
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/ghostfile"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
	"github.com/BaimPriyatna/repro/src/graph/store"
)

func TestGhostFile_New_Validation(t *testing.T) {
	s := store.NewMemStore()
	rm, _ := repairmap.New(s)

	// nil repair map
	_, err := ghostfile.New(nil, &analysis.HistoryChain{
		Snapshots: []*snapshot.Snapshot{{ID: "s1"}},
		Depth:     1,
	})
	if err == nil {
		t.Error("expected error for nil repair map, got nil")
	}

	// nil/empty history chain
	_, err = ghostfile.New(rm, nil)
	if err == nil {
		t.Error("expected error for nil history chain, got nil")
	}
	_, err = ghostfile.New(rm, &analysis.HistoryChain{})
	if err == nil {
		t.Error("expected error for empty history chain, got nil")
	}
}

func TestGhostFile_Analyze(t *testing.T) {
	s := store.NewMemStore()

	// Graph Entities
	// 1. app (service)
	// 2. config.json (used by app)
	// 3. dead_script.py (in graph, but 0 dependents)
	// 4. deleted_module.go (in graph, referenced by app, but missing in latest snapshot)
	entities := []*entity.Entity{
		{ID: "app", Kind: entity.KindService, Name: "web-server"},
		{ID: "config.json", Kind: entity.KindFile, Name: "config.json"},
		{ID: "dead_script.py", Kind: entity.KindFile, Name: "dead_script.py"},
		{ID: "deleted_module.go", Kind: entity.KindFile, Name: "deleted_module.go"},
	}
	for _, e := range entities {
		if err := s.AddEntity(e); err != nil {
			t.Fatalf("AddEntity failed: %v", err)
		}
	}

	// Graph Relations
	// app uses config.json
	// app imports deleted_module.go
	relations := []*relation.Relation{
		{ID: "r1", Kind: relation.KindUses, FromID: "app", ToID: "config.json"},
		{ID: "r2", Kind: relation.KindImports, FromID: "app", ToID: "deleted_module.go"},
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

	// History
	// Snapshot 1 (older): had config.json, dead_script.py, copy_script.py (same hash), deleted_module.go
	// Snapshot 2 (newer): has config.json, dead_script.py, copy_script.py (deleted_module.go is removed!)
	snap1 := &snapshot.Snapshot{
		ID:        "snap-1",
		Timestamp: time.Now().Add(-10 * time.Minute),
		Data: map[string]any{
			"files": map[string]any{
				"config.json":       "hash-config",
				"dead_script.py":    "hash-script-1",
				"copy_script.py":    "hash-script-1",
				"deleted_module.go": "hash-del",
			},
		},
	}
	snap2 := &snapshot.Snapshot{
		ID:        "snap-2",
		Timestamp: time.Now(),
		ParentID:  "snap-1",
		Data: map[string]any{
			"files": map[string]any{
				"config.json":    "hash-config",
				"dead_script.py": "hash-script-1",
				"copy_script.py": "hash-script-1",
				// deleted_module.go is missing here!
			},
		},
	}

	chain := &analysis.HistoryChain{
		Snapshots: []*snapshot.Snapshot{snap2, snap1},
		Depth:     2,
	}

	analyzer, err := ghostfile.New(rm, chain)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if analyzer.Name() != ghostfile.AnalyzerName {
		t.Errorf("Name() = %v, want %v", analyzer.Name(), ghostfile.AnalyzerName)
	}
	if analyzer.Version() != ghostfile.AnalyzerVersion {
		t.Errorf("Version() = %v, want %v", analyzer.Version(), ghostfile.AnalyzerVersion)
	}

	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.Status != coreanalysis.StatusFindings {
		t.Errorf("Status = %v, want %v", result.Status, coreanalysis.StatusFindings)
	}

	findingsBySubjectAndType := make(map[string]coreanalysis.Finding)
	for _, f := range result.Findings {
		if !f.HasEvidence() {
			t.Errorf("finding %s has no evidence (missing evidence)", f.ID)
		}
		key := string(f.Subject) + ":" + f.Type
		findingsBySubjectAndType[key] = f
	}

	// 1. deleted_module.go should have finding ghost_file_deleted_with_references
	if _, ok := findingsBySubjectAndType["deleted_module.go:"+ghostfile.FindingTypeDeletedWithReferences]; !ok {
		t.Errorf("expected finding ghost_file_deleted_with_references for deleted_module.go")
	}

	// 2. dead_script.py should have finding ghost_file_unused
	if _, ok := findingsBySubjectAndType["dead_script.py:"+ghostfile.FindingTypeUnused]; !ok {
		t.Errorf("expected finding ghost_file_unused for dead_script.py")
	}

	// 3. dead_script.py or copy_script.py should have finding ghost_file_copied
	if _, ok := findingsBySubjectAndType["dead_script.py:"+ghostfile.FindingTypeCopied]; !ok {
		if _, ok2 := findingsBySubjectAndType["copy_script.py:"+ghostfile.FindingTypeCopied]; !ok2 {
			t.Errorf("expected finding ghost_file_copied for duplicate hashes")
		}
	}
}
