package beforeafter_test

import (
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis/beforeafter"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

func TestNew(t *testing.T) {
	snapA := &snapshot.Snapshot{ID: snapshot.NewID(), Data: map[string]any{}}
	snapB := &snapshot.Snapshot{ID: snapshot.NewID(), Data: map[string]any{}}

	ba, err := beforeafter.New(snapA, snapB)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if ba == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_NilInputs(t *testing.T) {
	snap := &snapshot.Snapshot{ID: snapshot.NewID(), Data: map[string]any{}}

	_, err := beforeafter.New(nil, snap)
	if err == nil {
		t.Error("New(nil, snap) should return error")
	}

	_, err = beforeafter.New(snap, nil)
	if err == nil {
		t.Error("New(snap, nil) should return error")
	}
}

func TestAnalyzer_Interface(t *testing.T) {
	snapA := &snapshot.Snapshot{ID: snapshot.NewID(), Data: map[string]any{}}
	snapB := &snapshot.Snapshot{ID: snapshot.NewID(), Data: map[string]any{}}

	ba, _ := beforeafter.New(snapA, snapB)

	if ba.Name() != "beforeafter" {
		t.Errorf("Name() = %v, want beforeafter", ba.Name())
	}

	if ba.Version() == "" {
		t.Error("Version() should not be empty")
	}
}

func TestAnalyze_IdenticalSnapshots(t *testing.T) {
	data := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
		},
	}

	snapA := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      data,
	}

	snapB := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      data,
	}

	ba, _ := beforeafter.New(snapA, snapB)
	result, err := ba.Analyze()

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if result.Status != coreanalysis.StatusOK {
		t.Errorf("Status = %v, want %v", result.Status, coreanalysis.StatusOK)
	}

	// Identical snapshots should have no findings (only unchanged items)
	if len(result.Findings) != 0 {
		t.Errorf("Findings count = %d, want 0 for identical snapshots", len(result.Findings))
	}
}

func TestAnalyze_AddedField(t *testing.T) {
	dataA := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
		},
	}

	dataB := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
			"node": map[string]any{
				"version": "18.0.0",
			},
		},
	}

	snapA := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      dataA,
	}

	snapB := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      dataB,
	}

	ba, _ := beforeafter.New(snapA, snapB)
	result, err := ba.Analyze()

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if result.Status != coreanalysis.StatusFindings {
		t.Errorf("Status = %v, want %v", result.Status, coreanalysis.StatusFindings)
	}

	if len(result.Findings) == 0 {
		t.Error("should have detected added field")
	}

	// Check for field_added finding
	foundAdded := false
	for _, finding := range result.Findings {
		if finding.Type == "field_added" {
			foundAdded = true
			if finding.Confidence != coreanalysis.ConfidenceConfirmed {
				t.Errorf("Confidence = %v, want %v", finding.Confidence, coreanalysis.ConfidenceConfirmed)
			}
			if !finding.HasEvidence() {
				t.Error("Finding should have evidence")
			}
		}
	}

	if !foundAdded {
		t.Error("did not find field_added finding")
	}
}

func TestAnalyze_RemovedField(t *testing.T) {
	dataA := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
			"node": map[string]any{
				"version": "18.0.0",
			},
		},
	}

	dataB := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
		},
	}

	snapA := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      dataA,
	}

	snapB := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      dataB,
	}

	ba, _ := beforeafter.New(snapA, snapB)
	result, err := ba.Analyze()

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if result.Status != coreanalysis.StatusFindings {
		t.Errorf("Status = %v, want %v", result.Status, coreanalysis.StatusFindings)
	}

	// Check for field_removed finding
	foundRemoved := false
	for _, finding := range result.Findings {
		if finding.Type == "field_removed" {
			foundRemoved = true
			if finding.Severity == coreanalysis.SeverityInfo {
				t.Error("Removed fields should have higher severity than Info")
			}
		}
	}

	if !foundRemoved {
		t.Error("did not find field_removed finding")
	}
}

func TestAnalyze_ModifiedField(t *testing.T) {
	dataA := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
		},
	}

	dataB := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.22.0",
			},
		},
	}

	snapA := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      dataA,
	}

	snapB := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      dataB,
	}

	ba, _ := beforeafter.New(snapA, snapB)
	result, err := ba.Analyze()

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if result.Status != coreanalysis.StatusFindings {
		t.Errorf("Status = %v, want %v", result.Status, coreanalysis.StatusFindings)
	}

	// Check for field_modified finding
	foundModified := false
	for _, finding := range result.Findings {
		if finding.Type == "field_modified" {
			foundModified = true
		}
	}

	if !foundModified {
		t.Error("did not find field_modified finding")
	}
}

func TestAnalyze_ComplexChanges(t *testing.T) {
	dataA := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.21.0",
			},
			"node": map[string]any{
				"version": "18.0.0",
			},
		},
		"env": map[string]any{
			"NODE_ENV": "production",
		},
	}

	dataB := map[string]any{
		"runtime": map[string]any{
			"go": map[string]any{
				"version": "1.22.0", // modified
			},
			"python": map[string]any{ // added
				"version": "3.11.0",
			},
		},
		"env": map[string]any{
			"NODE_ENV": "production", // unchanged
			"DEBUG":    "true",       // added
		},
	}

	snapA := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      dataA,
	}

	snapB := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      dataB,
	}

	ba, _ := beforeafter.New(snapA, snapB)
	result, err := ba.Analyze()

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if len(result.Findings) == 0 {
		t.Error("should have detected multiple changes")
	}

	// Should have added, removed, and modified findings
	hasAdded := false
	hasRemoved := false
	hasModified := false

	for _, finding := range result.Findings {
		switch finding.Type {
		case "field_added":
			hasAdded = true
		case "field_removed":
			hasRemoved = true
		case "field_modified":
			hasModified = true
		}
	}

	if !hasAdded {
		t.Error("should have added findings")
	}
	if !hasRemoved {
		t.Error("should have removed findings")
	}
	if !hasModified {
		t.Error("should have modified findings")
	}
}

func TestAnalysisResult_Contract(t *testing.T) {
	snapA := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data: map[string]any{
			"test": "old",
		},
	}

	snapB := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data: map[string]any{
			"test": "new",
		},
	}

	ba, _ := beforeafter.New(snapA, snapB)
	result, err := ba.Analyze()

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	// Verify AnalysisResult contract
	if result.ID == "" {
		t.Error("result ID should not be empty")
	}

	if result.Analyzer != "beforeafter" {
		t.Errorf("Analyzer = %v, want beforeafter", result.Analyzer)
	}

	if result.Version == "" {
		t.Error("Version should not be empty")
	}

	if result.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}

	if len(result.RelatedSnapshots) != 2 {
		t.Errorf("RelatedSnapshots count = %d, want 2", len(result.RelatedSnapshots))
	}

	// Verify all findings have evidence
	for i, finding := range result.Findings {
		if !finding.HasEvidence() {
			t.Errorf("Finding[%d] has no evidence", i)
		}
	}
}

func TestGetSummary(t *testing.T) {
	dataA := map[string]any{
		"a": "value1",
		"b": "value2",
		"c": "value3",
	}

	dataB := map[string]any{
		"a": "changed", // modified
		"d": "new",     // added
		// b removed
		"c": "value3", // unchanged
	}

	snapA := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      dataA,
	}

	snapB := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now(),
		Data:      dataB,
	}

	ba, _ := beforeafter.New(snapA, snapB)
	result, _ := ba.Analyze()
	summary := ba.GetSummary(result)

	if summary.SnapshotA != snapA.ID {
		t.Errorf("SnapshotA = %v, want %v", summary.SnapshotA, snapA.ID)
	}

	if summary.SnapshotB != snapB.ID {
		t.Errorf("SnapshotB = %v, want %v", summary.SnapshotB, snapB.ID)
	}

	if !summary.HasChanges {
		t.Error("HasChanges should be true")
	}

	if summary.AddedCount == 0 {
		t.Error("should have added count")
	}

	if summary.RemovedCount == 0 {
		t.Error("should have removed count")
	}

	if summary.ModifiedCount == 0 {
		t.Error("should have modified count")
	}
}

func TestAnalyzer_Independence(t *testing.T) {
	snapA := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: map[string]any{"field": "old"},
	}

	snapB := &snapshot.Snapshot{
		ID:   snapshot.NewID(),
		Data: map[string]any{"field": "new"},
	}

	ba, err := beforeafter.New(snapA, snapB)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	result, err := ba.Analyze()
	if err != nil {
		t.Fatalf("Analyze() failed: %v", err)
	}

	if result == nil {
		t.Fatal("Analyze() returned nil result")
	}

	// Verify it produced valid findings
	if len(result.Findings) == 0 {
		t.Error("should have produced findings")
	}
}
