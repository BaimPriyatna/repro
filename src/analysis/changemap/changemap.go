// Package changemap maps changes across snapshot history with temporal context.
package changemap

import (
	"fmt"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/diff"
	"github.com/BaimPriyatna/repro/src/analysis/history"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

const (
	AnalyzerName    = "changemap"
	AnalyzerVersion = "1.0.0"
)

// ChangeMap provides temporal mapping of changes.
type ChangeMap struct {
	chain     *analysis.HistoryChain
	traverser *history.Traverser
}

// New creates a new ChangeMap analyzer.
func New(startSnapshot *snapshot.Snapshot, traverser *history.Traverser) (*ChangeMap, error) {
	if startSnapshot == nil || traverser == nil {
		return nil, analysis.ErrInvalidInput
	}

	chain, err := traverser.Traverse(analysis.HistoryQuery{
		StartID:      startSnapshot.ID,
		IncludeStart: true,
		MaxDepth:     20,
	})
	if err != nil {
		return nil, err
	}

	return &ChangeMap{chain: chain, traverser: traverser}, nil
}

func (cm *ChangeMap) Name() string    { return AnalyzerName }
func (cm *ChangeMap) Version() string { return AnalyzerVersion }

// Analyze maps changes across the snapshot timeline.
func (cm *ChangeMap) Analyze() (*coreanalysis.AnalysisResult, error) {
	result := &coreanalysis.AnalysisResult{
		ID:               coreanalysis.ResultID(fmt.Sprintf("changemap-%s", cm.chain.Newest().ID)),
		Analyzer:         AnalyzerName,
		Version:          AnalyzerVersion,
		Timestamp:        time.Now().UTC(),
		Status:           coreanalysis.StatusOK,
		Findings:         make([]coreanalysis.Finding, 0),
		Evidence:         make([]coreanalysis.DiagnosticEvidence, 0),
		RelatedSnapshots: make([]snapshot.ID, len(cm.chain.Snapshots)),
	}

	for i, snap := range cm.chain.Snapshots {
		result.RelatedSnapshots[i] = snap.ID
		result.Evidence = append(result.Evidence, coreanalysis.DiagnosticEvidence{
			Kind:      coreanalysis.EvidenceKindSnapshot,
			SourceID:  string(snap.ID),
			Timestamp: snap.Timestamp,
		})
	}

	// Map changes between consecutive snapshots
	for i := 0; i < len(cm.chain.Snapshots)-1; i++ {
		newer := cm.chain.Snapshots[i]
		older := cm.chain.Snapshots[i+1]

		diffResult, err := diff.Diff(older, newer)
		if err != nil {
			continue
		}

		changes := diff.FilterChanges(diffResult.Changes,
			analysis.ChangeTypeAdded,
			analysis.ChangeTypeRemoved,
			analysis.ChangeTypeModified)

		for j, change := range changes {
			finding := coreanalysis.Finding{
				ID:          coreanalysis.FindingID(fmt.Sprintf("changemap-%d-%d", i, j)),
				Severity:    coreanalysis.SeverityInfo,
				Type:        "temporal_change",
				Subject:     entity.ID(change.Path),
				Description: fmt.Sprintf("Field '%s' changed between %s and %s", change.Path, older.Timestamp.Format(time.RFC3339), newer.Timestamp.Format(time.RFC3339)),
				Confidence:  coreanalysis.ConfidenceConfirmed,
				Evidence: []coreanalysis.DiagnosticEvidence{
					{
						Kind:      coreanalysis.EvidenceKindSnapshot,
						SourceID:  string(older.ID),
						Field:     change.Path,
						Value:     change.OldValue,
						Timestamp: older.Timestamp,
					},
					{
						Kind:      coreanalysis.EvidenceKindSnapshot,
						SourceID:  string(newer.ID),
						Field:     change.Path,
						Value:     change.NewValue,
						Timestamp: newer.Timestamp,
					},
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	if len(result.Findings) > 0 {
		result.Status = coreanalysis.StatusFindings
	}

	return result, nil
}
