// Package drift detects configuration drift against a baseline snapshot.
package drift

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
	AnalyzerName    = "drift"
	AnalyzerVersion = "1.0.0"
)

// Drift detects gradual deviation from a baseline.
type Drift struct {
	baseline  *snapshot.Snapshot
	chain     *analysis.HistoryChain
	traverser *history.Traverser
}

// New creates a new Drift analyzer.
func New(baseline *snapshot.Snapshot, traverser *history.Traverser) (*Drift, error) {
	if baseline == nil || traverser == nil {
		return nil, analysis.ErrInvalidInput
	}
	return &Drift{baseline: baseline, traverser: traverser}, nil
}

func (d *Drift) Name() string    { return AnalyzerName }
func (d *Drift) Version() string { return AnalyzerVersion }

// Analyze detects drift from the baseline across the snapshot chain.
func (d *Drift) Analyze() (*coreanalysis.AnalysisResult, error) {
	// Traverse history from baseline
	chain, err := d.traverser.Traverse(analysis.HistoryQuery{
		StartID:      d.baseline.ID,
		IncludeStart: true,
		MaxDepth:     10, // reasonable limit
	})
	if err != nil {
		return nil, err
	}
	d.chain = chain

	result := &coreanalysis.AnalysisResult{
		ID:        coreanalysis.ResultID(fmt.Sprintf("drift-%s", d.baseline.ID)),
		Analyzer:  AnalyzerName,
		Version:   AnalyzerVersion,
		Timestamp: time.Now().UTC(),
		Status:    coreanalysis.StatusOK,
		Findings:  make([]coreanalysis.Finding, 0),
		Evidence: []coreanalysis.DiagnosticEvidence{
			{
				Kind:      coreanalysis.EvidenceKindSnapshot,
				SourceID:  string(d.baseline.ID),
				Timestamp: d.baseline.Timestamp,
			},
		},
		RelatedSnapshots: make([]snapshot.ID, len(chain.Snapshots)),
	}

	for i, snap := range chain.Snapshots {
		result.RelatedSnapshots[i] = snap.ID
	}

	// Compare each snapshot in chain with baseline
	for i := 1; i < len(chain.Snapshots); i++ {
		findings := d.compareWithBaseline(chain.Snapshots[i], i)
		result.Findings = append(result.Findings, findings...)
	}

	if len(result.Findings) > 0 {
		result.Status = coreanalysis.StatusFindings
	}

	return result, nil
}

func (d *Drift) compareWithBaseline(snap *snapshot.Snapshot, index int) []coreanalysis.Finding {
	diffResult, err := diff.Diff(d.baseline, snap)
	if err != nil {
		return nil
	}

	findings := make([]coreanalysis.Finding, 0)
	changes := diff.FilterChanges(diffResult.Changes, analysis.ChangeTypeModified)

	for _, change := range changes {
		finding := coreanalysis.Finding{
			ID:          coreanalysis.FindingID(fmt.Sprintf("drift-%d-%s", index, change.Path)),
			Severity:    coreanalysis.SeverityLow,
			Type:        "gradual_drift",
			Subject:     entity.ID(change.Path),
			Description: fmt.Sprintf("Field '%s' drifted from baseline", change.Path),
			Confidence:  coreanalysis.ConfidenceConfirmed,
			Evidence: []coreanalysis.DiagnosticEvidence{
				{
					Kind:      coreanalysis.EvidenceKindDiff,
					SourceID:  string(snap.ID),
					Field:     change.Path,
					Value:     change.NewValue,
					Timestamp: snap.Timestamp,
				},
			},
		}
		findings = append(findings, finding)
	}

	return findings
}
