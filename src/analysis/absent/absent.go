// Package absent detects entities that were present before but are missing now.
package absent

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
	AnalyzerName    = "absent"
	AnalyzerVersion = "1.0.0"
)

// Absent detects missing entities.
type Absent struct {
	before *snapshot.Snapshot
	after  *snapshot.Snapshot
}

// New creates a new Absent analyzer.
func New(before, after *snapshot.Snapshot) (*Absent, error) {
	if before == nil || after == nil {
		return nil, analysis.ErrInvalidInput
	}
	return &Absent{before: before, after: after}, nil
}

func (a *Absent) Name() string    { return AnalyzerName }
func (a *Absent) Version() string { return AnalyzerVersion }

// Analyze identifies entities present in 'before' but missing in 'after'.
func (a *Absent) Analyze() (*coreanalysis.AnalysisResult, error) {
	diffResult, err := diff.Diff(a.before, a.after)
	if err != nil {
		return nil, err
	}

	result := &coreanalysis.AnalysisResult{
		ID:        coreanalysis.ResultID(fmt.Sprintf("absent-%s-%s", a.before.ID, a.after.ID)),
		Analyzer:  AnalyzerName,
		Version:   AnalyzerVersion,
		Timestamp: time.Now().UTC(),
		Status:    coreanalysis.StatusOK,
		Findings:  make([]coreanalysis.Finding, 0),
		Evidence: []coreanalysis.DiagnosticEvidence{
			{Kind: coreanalysis.EvidenceKindSnapshot, SourceID: string(a.before.ID), Timestamp: a.before.Timestamp},
			{Kind: coreanalysis.EvidenceKindSnapshot, SourceID: string(a.after.ID), Timestamp: a.after.Timestamp},
		},
		RelatedSnapshots: []snapshot.ID{a.before.ID, a.after.ID},
	}

	// Focus on removed items
	removed := diff.FilterChanges(diffResult.Changes, analysis.ChangeTypeRemoved)
	for i, change := range removed {
		finding := coreanalysis.Finding{
			ID:          coreanalysis.FindingID(fmt.Sprintf("absent-%d", i)),
			Severity:    coreanalysis.SeverityMedium,
			Type:        "entity_absent",
			Subject:     entity.ID(change.Path),
			Description: fmt.Sprintf("Entity '%s' was present before but is now missing", change.Path),
			Confidence:  coreanalysis.ConfidenceConfirmed,
			Evidence: []coreanalysis.DiagnosticEvidence{
				{
					Kind:      coreanalysis.EvidenceKindSnapshot,
					SourceID:  string(a.before.ID),
					Field:     change.Path,
					Value:     change.OldValue,
					Timestamp: a.before.Timestamp,
				},
			},
		}
		result.Findings = append(result.Findings, finding)
	}

	if len(result.Findings) > 0 {
		result.Status = coreanalysis.StatusFindings
	}

	return result, nil
}
