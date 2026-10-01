// Package beforeafter compares two snapshots for added, removed, modified, and unchanged items.
package beforeafter

import (
	"fmt"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/diff"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

const (
	AnalyzerName    = "beforeafter"
	AnalyzerVersion = "1.0.0"
)

// BeforeAfter compares two snapshots and produces an AnalysisResult.
type BeforeAfter struct {
	snapshotA *snapshot.Snapshot
	snapshotB *snapshot.Snapshot
}

// New creates a new BeforeAfter analyzer for comparing two snapshots.
func New(snapshotA, snapshotB *snapshot.Snapshot) (*BeforeAfter, error) {
	if snapshotA == nil || snapshotB == nil {
		return nil, analysis.ErrInvalidInput
	}

	return &BeforeAfter{
		snapshotA: snapshotA,
		snapshotB: snapshotB,
	}, nil
}

// Name returns the analyzer identifier.
func (ba *BeforeAfter) Name() string {
	return AnalyzerName
}

// Version returns the analyzer version.
func (ba *BeforeAfter) Version() string {
	return AnalyzerVersion
}

// Analyze performs the comparison and returns an AnalysisResult.
func (ba *BeforeAfter) Analyze() (*coreanalysis.AnalysisResult, error) {
	// Perform the diff
	diffResult, err := diff.Diff(ba.snapshotA, ba.snapshotB)
	if err != nil {
		return nil, err
	}

	// Generate result ID
	resultID := coreanalysis.ResultID(fmt.Sprintf("beforeafter-%s-%s", ba.snapshotA.ID, ba.snapshotB.ID))

	// Build the analysis result
	result := &coreanalysis.AnalysisResult{
		ID:        resultID,
		Analyzer:  AnalyzerName,
		Version:   AnalyzerVersion,
		Timestamp: time.Now().UTC(),
		Status:    coreanalysis.StatusOK,
		Findings:  make([]coreanalysis.Finding, 0),
		Evidence: []coreanalysis.DiagnosticEvidence{
			{
				Kind:      coreanalysis.EvidenceKindSnapshot,
				SourceID:  string(ba.snapshotA.ID),
				Timestamp: ba.snapshotA.Timestamp,
			},
			{
				Kind:      coreanalysis.EvidenceKindSnapshot,
				SourceID:  string(ba.snapshotB.ID),
				Timestamp: ba.snapshotB.Timestamp,
			},
		},
		RelatedSnapshots: []snapshot.ID{ba.snapshotA.ID, ba.snapshotB.ID},
	}

	// Generate findings from changes
	findings := ba.generateFindings(diffResult)
	result.Findings = findings

	// Set status based on findings
	if len(findings) > 0 {
		result.Status = coreanalysis.StatusFindings
	}

	return result, nil
}

// generateFindings converts diff changes into structured findings.
func (ba *BeforeAfter) generateFindings(diffResult *analysis.DiffResult) []coreanalysis.Finding {
	findings := make([]coreanalysis.Finding, 0)

	// Filter to only changes (exclude unchanged)
	changes := diff.FilterChanges(diffResult.Changes,
		analysis.ChangeTypeAdded,
		analysis.ChangeTypeRemoved,
		analysis.ChangeTypeModified,
	)

	for i, change := range changes {
		finding := ba.changeToFinding(i, change)
		findings = append(findings, finding)
	}

	return findings
}

// changeToFinding converts a single Change into a Finding.
func (ba *BeforeAfter) changeToFinding(index int, change analysis.Change) coreanalysis.Finding {
	findingID := coreanalysis.FindingID(fmt.Sprintf("%s-%d", ba.Name(), index))

	// Determine severity and type based on change type
	var severity coreanalysis.Severity
	var findingType string
	var description string

	switch change.Type {
	case analysis.ChangeTypeAdded:
		severity = coreanalysis.SeverityInfo
		findingType = "field_added"
		description = fmt.Sprintf("Field '%s' was added with value: %v", change.Path, change.NewValue)

	case analysis.ChangeTypeRemoved:
		severity = coreanalysis.SeverityMedium
		findingType = "field_removed"
		description = fmt.Sprintf("Field '%s' was removed (previous value: %v)", change.Path, change.OldValue)

	case analysis.ChangeTypeModified:
		severity = coreanalysis.SeverityMedium
		findingType = "field_modified"
		description = fmt.Sprintf("Field '%s' changed from %v to %v", change.Path, change.OldValue, change.NewValue)

	default:
		severity = coreanalysis.SeverityInfo
		findingType = "unknown_change"
		description = fmt.Sprintf("Field '%s' changed", change.Path)
	}

	// Create evidence for this finding
	evidence := []coreanalysis.DiagnosticEvidence{
		{
			Kind:      coreanalysis.EvidenceKindDiff,
			SourceID:  fmt.Sprintf("diff-%s-%s", ba.snapshotA.ID, ba.snapshotB.ID),
			Field:     change.Path,
			Value:     change.OldValue,
			Timestamp: ba.snapshotA.Timestamp,
		},
		{
			Kind:      coreanalysis.EvidenceKindDiff,
			SourceID:  fmt.Sprintf("diff-%s-%s", ba.snapshotA.ID, ba.snapshotB.ID),
			Field:     change.Path,
			Value:     change.NewValue,
			Timestamp: ba.snapshotB.Timestamp,
		},
	}

	return coreanalysis.Finding{
		ID:          findingID,
		Severity:    severity,
		Type:        findingType,
		Subject:     entity.ID(change.Path),
		Description: description,
		Confidence:  coreanalysis.ConfidenceConfirmed, // direct observation
		Evidence:    evidence,
	}
}

// Summary returns a human-readable summary of the comparison.
type Summary struct {
	SnapshotA      snapshot.ID `json:"snapshot_a"`
	SnapshotB      snapshot.ID `json:"snapshot_b"`
	AddedCount     int         `json:"added_count"`
	RemovedCount   int         `json:"removed_count"`
	ModifiedCount  int         `json:"modified_count"`
	UnchangedCount int         `json:"unchanged_count"`
	HasChanges     bool        `json:"has_changes"`
}

// GetSummary returns a high-level summary of the comparison.
func (ba *BeforeAfter) GetSummary(result *coreanalysis.AnalysisResult) Summary {
	summary := Summary{
		SnapshotA: ba.snapshotA.ID,
		SnapshotB: ba.snapshotB.ID,
	}

	for _, finding := range result.Findings {
		switch finding.Type {
		case "field_added":
			summary.AddedCount++
		case "field_removed":
			summary.RemovedCount++
		case "field_modified":
			summary.ModifiedCount++
		}
	}

	summary.HasChanges = summary.AddedCount > 0 || summary.RemovedCount > 0 || summary.ModifiedCount > 0

	return summary
}
