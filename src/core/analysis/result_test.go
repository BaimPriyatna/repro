package analysis_test

import (
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// AnalysisResult helpers

func TestAnalysisResult_HasFindings_Empty(t *testing.T) {
	r := &analysis.AnalysisResult{
		ID:        "res-001",
		Analyzer:  "beforeafter",
		Version:   "1.0.0",
		Timestamp: time.Now(),
		Status:    analysis.StatusOK,
	}
	if r.HasFindings() {
		t.Error("HasFindings() should be false when Findings is empty")
	}
	if r.FindingCount() != 0 {
		t.Errorf("FindingCount() = %d; want 0", r.FindingCount())
	}
}

func TestAnalysisResult_HasFindings_Populated(t *testing.T) {
	r := &analysis.AnalysisResult{
		ID:       "res-002",
		Analyzer: "drift",
		Status:   analysis.StatusFindings,
		Findings: []analysis.Finding{
			{
				ID:         "find-001",
				Severity:   analysis.SeverityHigh,
				Type:       "version_changed",
				Subject:    entity.ID("pkg-go"),
				Confidence: analysis.ConfidenceConfirmed,
				Evidence: []analysis.DiagnosticEvidence{
					{Kind: analysis.EvidenceKindSnapshot, SourceID: "snap-001"},
				},
			},
		},
	}
	if !r.HasFindings() {
		t.Error("HasFindings() should be true when Findings is non-empty")
	}
	if r.FindingCount() != 1 {
		t.Errorf("FindingCount() = %d; want 1", r.FindingCount())
	}
}

func TestAnalysisResult_IsError(t *testing.T) {
	r := &analysis.AnalysisResult{Status: analysis.StatusError}
	if !r.IsError() {
		t.Error("IsError() should be true when Status is StatusError")
	}
}

func TestAnalysisResult_IsNotError(t *testing.T) {
	r := &analysis.AnalysisResult{Status: analysis.StatusOK}
	if r.IsError() {
		t.Error("IsError() should be false when Status is not StatusError")
	}
}

// Finding evidence guard

func TestFinding_HasEvidence_True(t *testing.T) {
	f := &analysis.Finding{
		Evidence: []analysis.DiagnosticEvidence{
			{Kind: analysis.EvidenceKindEvent, SourceID: "evt-42"},
		},
	}
	if !f.HasEvidence() {
		t.Error("HasEvidence() should be true when Evidence is non-empty")
	}
}

func TestFinding_HasEvidence_False(t *testing.T) {
	f := &analysis.Finding{}
	if f.HasEvidence() {
		t.Error("HasEvidence() should be false when Evidence is empty")
	}
}

// Confidence constants

func TestConfidenceConstants_NonEmpty(t *testing.T) {
	confs := []analysis.Confidence{
		analysis.ConfidenceConfirmed,
		analysis.ConfidenceHigh,
		analysis.ConfidenceMedium,
		analysis.ConfidenceLow,
		analysis.ConfidenceUnknown,
	}
	for _, c := range confs {
		if c == "" {
			t.Errorf("Confidence constant must not be empty string")
		}
	}
}

// Severity constants

func TestSeverityConstants_NonEmpty(t *testing.T) {
	sevs := []analysis.Severity{
		analysis.SeverityCritical,
		analysis.SeverityHigh,
		analysis.SeverityMedium,
		analysis.SeverityLow,
		analysis.SeverityInfo,
	}
	for _, s := range sevs {
		if s == "" {
			t.Errorf("Severity constant must not be empty string")
		}
	}
}

// EvidenceKind constants

func TestEvidenceKindConstants_NonEmpty(t *testing.T) {
	kinds := []analysis.EvidenceKind{
		analysis.EvidenceKindSnapshot,
		analysis.EvidenceKindEvent,
		analysis.EvidenceKindDiff,
		analysis.EvidenceKindFile,
		analysis.EvidenceKindPackage,
		analysis.EvidenceKindConfig,
		analysis.EvidenceKindRelation,
		analysis.EvidenceKindCommand,
		analysis.EvidenceKindGitCommit,
	}
	for _, k := range kinds {
		if k == "" {
			t.Errorf("EvidenceKind constant must not be empty string")
		}
	}
}

// RelatedSnapshots linkage

func TestAnalysisResult_RelatedSnapshots(t *testing.T) {
	r := &analysis.AnalysisResult{
		Analyzer: "beforeafter",
		Status:   analysis.StatusOK,
		RelatedSnapshots: []snapshot.ID{
			"snap-before",
			"snap-after",
		},
	}
	if len(r.RelatedSnapshots) != 2 {
		t.Errorf("RelatedSnapshots len = %d; want 2", len(r.RelatedSnapshots))
	}
}

// Severity ≠ Confidence — both axes must be independently settable

func TestFinding_SeverityAndConfidenceAreIndependent(t *testing.T) {
	f := analysis.Finding{
		ID:          "find-x",
		Severity:    analysis.SeverityCritical,
		Confidence:  analysis.ConfidenceLow,
		Type:        "possible_cause",
		Description: "Critical severity but low confidence",
	}
	if f.Severity == analysis.Severity(f.Confidence) {
		t.Error("Severity and Confidence must be independent axes with distinct types")
	}
}
