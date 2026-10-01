package explaindiff_test

import (
	"strings"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/presentation/explaindiff"
)

func sampleWhyBrokenResult() *coreanalysis.AnalysisResult {
	return &coreanalysis.AnalysisResult{
		ID:        "whybroken-s-a-s-b",
		Analyzer:  "whybroken",
		Version:   "1.0.0",
		Timestamp: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
		Status:    coreanalysis.StatusFindings,
		Findings: []coreanalysis.Finding{
			{
				ID:          "whybroken-confirmed_change-shared-lib",
				Severity:    coreanalysis.SeverityCritical,
				Type:        "confirmed_change",
				Subject:     entity.ID("shared-lib"),
				Description: "shared-lib version changed with corroborating event",
				Confidence:  coreanalysis.ConfidenceConfirmed,
				Evidence: []coreanalysis.DiagnosticEvidence{
					{Kind: coreanalysis.EvidenceKindDiff, SourceID: "s-a->s-b", Field: "packages.shared-lib.version"},
					{Kind: coreanalysis.EvidenceKindEvent, SourceID: "evt-1", Field: "subject", Value: "shared-lib"},
					{Kind: coreanalysis.EvidenceKindRelation, SourceID: "shared-lib", Field: "graph_link", Value: "transitive"},
				},
			},
			{
				ID:          "whybroken-unknown-evt-orphan",
				Severity:    coreanalysis.SeverityInfo,
				Type:        "unknown",
				Subject:     entity.ID("curl https://example.com"),
				Description: "orphan event with no Diff or graph link",
				Confidence:  coreanalysis.ConfidenceUnknown,
				Evidence: []coreanalysis.DiagnosticEvidence{
					{Kind: coreanalysis.EvidenceKindEvent, SourceID: "evt-orphan", Field: "subject"},
				},
			},
		},
		Evidence: []coreanalysis.DiagnosticEvidence{
			{Kind: coreanalysis.EvidenceKindSnapshot, SourceID: "s-a", Field: "before"},
			{Kind: coreanalysis.EvidenceKindSnapshot, SourceID: "s-b", Field: "after"},
		},
		RelatedSnapshots: []snapshot.ID{"s-a", "s-b"},
		RelatedEvents:    []event.ID{"evt-1", "evt-orphan"},
		RelatedEntities:  []entity.ID{"shared-lib"},
	}
}

func TestNew_Validation(t *testing.T) {
	if _, err := explaindiff.New(nil); err == nil {
		t.Error("expected error for nil result")
	}

	bad := sampleWhyBrokenResult()
	bad.Analyzer = "impact"
	if _, err := explaindiff.New(bad); err == nil {
		t.Error("expected error for non-whybroken analyzer")
	}

	ed, err := explaindiff.New(sampleWhyBrokenResult())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if ed == nil {
		t.Fatal("New returned nil")
	}
}

func TestExplain_CitesWhyBrokenEvidence(t *testing.T) {
	ed, err := explaindiff.New(sampleWhyBrokenResult(),
		explaindiff.WithContext("web-app failing health checks"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	exp, err := ed.Explain()
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}

	if exp.SourceResultID != "whybroken-s-a-s-b" {
		t.Errorf("SourceResultID = %q", exp.SourceResultID)
	}
	if exp.SourceAnalyzer != "whybroken" {
		t.Errorf("SourceAnalyzer = %q", exp.SourceAnalyzer)
	}
	if !exp.HasEvidence() {
		t.Fatal("explanation must cite evidence")
	}
	if exp.Context != "web-app failing health checks" {
		t.Errorf("Context = %q", exp.Context)
	}
	if len(exp.Findings) != 2 {
		t.Fatalf("Findings = %d, want 2", len(exp.Findings))
	}

	ids := exp.FindingIDs()
	if ids[0] != "whybroken-confirmed_change-shared-lib" {
		t.Errorf("first finding ID = %q", ids[0])
	}

	// Every explained finding must retain evidence refs
	for _, f := range exp.Findings {
		if len(f.Evidence) == 0 {
			t.Errorf("finding %s has no evidence refs", f.FindingID)
		}
		if f.FindingID == "" {
			t.Error("finding ID must be preserved")
		}
	}

	if len(exp.Observed) == 0 {
		t.Error("Observed must not be empty when findings exist")
	}
	if len(exp.PotentialEffects) == 0 {
		t.Error("PotentialEffects must not be empty when findings exist")
	}
}

func TestExplain_WithDiffEnrichment(t *testing.T) {
	diffResult := &analysis.DiffResult{
		SnapshotA: "s-a",
		SnapshotB: "s-b",
		Summary:   analysis.DiffSummary{Added: 1, Removed: 0, Modified: 2},
		Changes: []analysis.Change{
			{Type: analysis.ChangeTypeModified, Path: "packages.shared-lib.version"},
		},
	}

	ed, err := explaindiff.New(sampleWhyBrokenResult(), explaindiff.WithDiff(diffResult))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	exp, err := ed.Explain()
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}

	found := false
	for _, line := range exp.Observed {
		if strings.Contains(line, "Snapshot Diff") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Diff summary in Observed, got %v", exp.Observed)
	}
}

func TestExplain_EmptyFindings(t *testing.T) {
	r := sampleWhyBrokenResult()
	r.Findings = nil
	r.Status = coreanalysis.StatusOK

	ed, err := explaindiff.New(r)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	exp, err := ed.Explain()
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if len(exp.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(exp.Findings))
	}
	// Result-level snapshot evidence still cited
	if !exp.HasEvidence() {
		t.Error("empty-findings explanation should still cite result-level evidence")
	}
}

func TestExplain_DoesNotReclassifyFindings(t *testing.T) {
	src := sampleWhyBrokenResult()
	ed, _ := explaindiff.New(src)
	exp, err := ed.Explain()
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}

	for i, f := range exp.Findings {
		orig := src.Findings[i]
		if f.Type != orig.Type {
			t.Errorf("type mutated: %q → %q", orig.Type, f.Type)
		}
		if f.Confidence != orig.Confidence {
			t.Errorf("confidence mutated: %q → %q", orig.Confidence, f.Confidence)
		}
		if f.FindingID != orig.ID {
			t.Errorf("finding ID mutated: %q → %q", orig.ID, f.FindingID)
		}
	}
}
