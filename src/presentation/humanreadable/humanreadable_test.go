package humanreadable_test

import (
	"strings"
	"testing"
	"time"

	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/presentation/explaindiff"
	"github.com/BaimPriyatna/repro/src/presentation/humanreadable"
)

func sampleResult() *coreanalysis.AnalysisResult {
	return &coreanalysis.AnalysisResult{
		ID:        "whybroken-chain",
		Analyzer:  "whybroken",
		Version:   "1.0.0",
		Timestamp: time.Date(2026, 9, 30, 18, 0, 0, 0, time.UTC),
		Status:    coreanalysis.StatusFindings,
		Findings: []coreanalysis.Finding{
			{
				ID:          "f-confirmed",
				Severity:    coreanalysis.SeverityCritical,
				Type:        "confirmed_change",
				Subject:     entity.ID("auth-service"),
				Description: "auth-service config changed with event",
				Confidence:  coreanalysis.ConfidenceConfirmed,
				Evidence: []coreanalysis.DiagnosticEvidence{
					{Kind: coreanalysis.EvidenceKindDiff, SourceID: "s1->s2", Field: "config.auth"},
					{Kind: coreanalysis.EvidenceKindEvent, SourceID: "evt-auth"},
				},
			},
			{
				ID:          "f-likely",
				Severity:    coreanalysis.SeverityHigh,
				Type:        "likely_contributor",
				Subject:     entity.ID("shared-lib"),
				Description: "shared-lib is a direct dependency that changed",
				Confidence:  coreanalysis.ConfidenceHigh,
				Evidence: []coreanalysis.DiagnosticEvidence{
					{Kind: coreanalysis.EvidenceKindDiff, SourceID: "s1->s2", Field: "packages.shared-lib"},
					{Kind: coreanalysis.EvidenceKindRelation, SourceID: "shared-lib", Field: "graph_link"},
				},
			},
		},
		Evidence: []coreanalysis.DiagnosticEvidence{
			{Kind: coreanalysis.EvidenceKindSnapshot, SourceID: "s1", Field: "before"},
			{Kind: coreanalysis.EvidenceKindSnapshot, SourceID: "s2", Field: "after"},
		},
		RelatedSnapshots: []snapshot.ID{"s1", "s2"},
		RelatedEvents:    []event.ID{"evt-auth"},
	}
}

func mustExplain(t *testing.T) *explaindiff.Explanation {
	t.Helper()
	ed, err := explaindiff.New(sampleResult(), explaindiff.WithContext("API latency spike"))
	if err != nil {
		t.Fatalf("explaindiff.New: %v", err)
	}
	exp, err := ed.Explain()
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	return exp
}

func TestNew_Validation(t *testing.T) {
	if _, err := humanreadable.New(nil); err == nil {
		t.Error("expected error for nil explanation")
	}
}

func TestRender_PlainLanguage(t *testing.T) {
	exp := mustExplain(t)
	hr, err := humanreadable.New(exp)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	diag, err := hr.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if diag.Text == "" {
		t.Fatal("Text must not be empty")
	}
	if !strings.Contains(diag.Text, "Observed:") {
		t.Error("Text should contain Observed section")
	}
	if !strings.Contains(diag.Text, "Evidence:") {
		t.Error("Text should contain Evidence section")
	}
	if !strings.Contains(diag.Text, "API latency spike") {
		t.Error("Text should include context")
	}
	if !strings.Contains(diag.Text, "f-confirmed") {
		t.Error("Text must preserve finding IDs")
	}
}

func TestRender_DoesNotAlterDiagnosis(t *testing.T) {
	exp := mustExplain(t)
	origFindingCount := len(exp.Findings)
	origTypes := make([]string, len(exp.Findings))
	for i, f := range exp.Findings {
		origTypes[i] = f.Type
	}

	hr, _ := humanreadable.New(exp)
	diag, err := hr.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Explanation unchanged
	if len(exp.Findings) != origFindingCount {
		t.Error("Render mutated explanation findings count")
	}
	for i, f := range exp.Findings {
		if f.Type != origTypes[i] {
			t.Errorf("Render mutated finding type %q → %q", origTypes[i], f.Type)
		}
	}

	// Diagnosis finding IDs match explanation exactly
	if len(diag.FindingIDs) != origFindingCount {
		t.Fatalf("FindingIDs len = %d, want %d", len(diag.FindingIDs), origFindingCount)
	}
	for i, id := range diag.FindingIDs {
		if id != string(exp.Findings[i].FindingID) {
			t.Errorf("FindingIDs[%d] = %q, want %q", i, id, exp.Findings[i].FindingID)
		}
	}
}

func TestRender_TracesToWhyBrokenEvidence(t *testing.T) {
	exp := mustExplain(t)
	hr, _ := humanreadable.New(exp)
	diag, err := hr.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !diag.TracesToResult("whybroken-chain") {
		t.Error("diagnosis must trace to WhyBroken result ID")
	}
	if !diag.TracesToExplanation(exp.ID) {
		t.Error("diagnosis must trace to Explanation ID")
	}
	if len(diag.EvidenceSourceIDs) == 0 {
		t.Error("diagnosis must list evidence source IDs")
	}

	// Evidence from WhyBroken must appear in text
	if !strings.Contains(diag.Text, "Snapshot s1") || !strings.Contains(diag.Text, "Snapshot s2") {
		t.Error("diagnosis must cite related snapshots")
	}
	if !strings.Contains(diag.Text, "Event evt-auth") {
		t.Error("diagnosis must cite related events")
	}
}

func TestChain_WhyBrokenToExplainDiffToHumanReadable(t *testing.T) {
	// Full chain
	result := sampleResult()

	ed, err := explaindiff.New(result)
	if err != nil {
		t.Fatalf("explaindiff.New: %v", err)
	}
	exp, err := ed.Explain()
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}

	hr, err := humanreadable.New(exp)
	if err != nil {
		t.Fatalf("humanreadable.New: %v", err)
	}
	diag, err := hr.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// : every output traces back to WhyBroken evidence
	if diag.SourceResultID != string(result.ID) {
		t.Errorf("SourceResultID = %q, want %q", diag.SourceResultID, result.ID)
	}
	for _, f := range result.Findings {
		found := false
		for _, id := range diag.FindingIDs {
			if id == string(f.ID) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("finding %s missing from diagnosis", f.ID)
		}
		if !strings.Contains(diag.Text, string(f.ID)) {
			t.Errorf("finding %s not present in diagnosis text", f.ID)
		}
	}
	if !exp.HasEvidence() {
		t.Error("explanation must have evidence")
	}
}
