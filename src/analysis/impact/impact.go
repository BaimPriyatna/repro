// Package impact estimates the blast radius of a change via reverse dependency traversal.
// Findings are labeled "potentially affected" — dependents are not assumed broken.
package impact

import (
	"fmt"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
)

const (
	// AnalyzerName is the unique identifier for the Impact analyzer.
	AnalyzerName = "impact"

	// AnalyzerVersion is the version of this analyzer implementation.
	AnalyzerVersion = "1.0.0"

	// DefaultMaxDepth is the default maximum traversal depth for impact analysis.
	DefaultMaxDepth = 50

	// FindingTypePotentiallyAffected is the finding type for potentially affected components.
	FindingTypePotentiallyAffected = "potentially_affected"
)

// Impact performs impact analysis on a dependency graph for given changed entities.
type Impact struct {
	repairMap       *repairmap.RepairMap
	changedEntities []entity.ID
	maxDepth        int
}

// Option configures the Impact analyzer.
type Option func(*Impact)

// WithMaxDepth sets the maximum traversal depth.
func WithMaxDepth(depth int) Option {
	return func(i *Impact) {
		if depth > 0 {
			i.maxDepth = depth
		}
	}
}

// New creates a new Impact analyzer for the given RepairMap and changed entity IDs.
func New(rm *repairmap.RepairMap, changedEntities []entity.ID, opts ...Option) (*Impact, error) {
	if rm == nil {
		return nil, fmt.Errorf("impact: repair map must not be nil")
	}
	if len(changedEntities) == 0 {
		return nil, fmt.Errorf("impact: at least one changed entity ID is required: %w", analysis.ErrInvalidInput)
	}

	imp := &Impact{
		repairMap:       rm,
		changedEntities: changedEntities,
		maxDepth:        DefaultMaxDepth,
	}

	for _, opt := range opts {
		opt(imp)
	}

	return imp, nil
}

// Name returns the analyzer identifier.
func (i *Impact) Name() string {
	return AnalyzerName
}

// Version returns the analyzer version.
func (i *Impact) Version() string {
	return AnalyzerVersion
}

// Analyze executes the impact analysis and produces an AnalysisResult.
func (i *Impact) Analyze() (*coreanalysis.AnalysisResult, error) {
	resultID := coreanalysis.ResultID(fmt.Sprintf("impact-%d", time.Now().UTC().UnixNano()))

	result := &coreanalysis.AnalysisResult{
		ID:        resultID,
		Analyzer:  AnalyzerName,
		Version:   AnalyzerVersion,
		Timestamp: time.Now().UTC(),
		Status:    coreanalysis.StatusOK,
		Findings:  make([]coreanalysis.Finding, 0),
		Evidence:  make([]coreanalysis.DiagnosticEvidence, 0),
	}

	// Track all impacted entities across all changed sources
	seenImpacted := make(map[entity.ID]bool)

	for _, changedID := range i.changedEntities {
		changedEntity, err := i.repairMap.GetEntity(changedID)
		if err != nil {
			// Entity not found in graph, record evidence of missing source
			result.Evidence = append(result.Evidence, coreanalysis.DiagnosticEvidence{
				Kind:     coreanalysis.EvidenceKindRelation,
				SourceID: string(changedID),
				Field:    "error",
				Value:    "changed entity not found in graph",
			})
			continue
		}

		changedName := changedEntity.Name
		if changedName == "" {
			changedName = string(changedID)
		}

		// Add base evidence for the changed entity
		result.Evidence = append(result.Evidence, coreanalysis.DiagnosticEvidence{
			Kind:     evidenceKindForEntity(changedEntity),
			SourceID: string(changedID),
			Field:    "changed_entity",
			Value:    string(changedEntity.Kind),
		})

		impactedEntities, err := i.repairMap.ImpactSet(changedID, i.maxDepth)
		if err != nil {
			return nil, fmt.Errorf("impact: failed to compute impact set for %s: %w", changedID, err)
		}

		for _, impacted := range impactedEntities {
			if impacted.ID == changedID {
				continue
			}

			// Calculate shortest path / distance from changed to impacted
			path, _ := i.repairMap.ShortestPath(impacted.ID, changedID, "")
			distance := len(path) - 1
			if distance <= 0 {
				distance = 1
			}

			// Direct dependents get higher confidence; transitive get medium/low.
			confidence := coreanalysis.ConfidenceMedium
			severity := coreanalysis.SeverityMedium
			if distance == 1 {
				confidence = coreanalysis.ConfidenceHigh
				severity = coreanalysis.SeverityHigh
			} else if distance > 3 {
				confidence = coreanalysis.ConfidenceLow
				severity = coreanalysis.SeverityLow
			}

			impactedName := impacted.Name
			if impactedName == "" {
				impactedName = string(impacted.ID)
			}

			findingID := coreanalysis.FindingID(fmt.Sprintf("impact-%s-%s", changedID, impacted.ID))

			// Build evidence for this finding pointing to the dependency relation
			findingEvidence := []coreanalysis.DiagnosticEvidence{
				{
					Kind:     evidenceKindForEntity(impacted),
					SourceID: string(impacted.ID),
					Field:    "entity.kind",
					Value:    string(impacted.Kind),
				},
				{
					Kind:     coreanalysis.EvidenceKindRelation,
					SourceID: string(changedID),
					Field:    "dependency_distance",
					Value:    distance,
				},
			}

			// Path relations list
			relatedEntities := []entity.ID{changedID}
			if len(path) > 0 {
				relatedEntities = path
			}

			finding := coreanalysis.Finding{
				ID:          findingID,
				Severity:    severity,
				Type:        FindingTypePotentiallyAffected,
				Subject:     impacted.ID,
				Description: fmt.Sprintf("Component '%s' (%s) is potentially affected by change in '%s' (distance: %d)", impactedName, impacted.Kind, changedName, distance),
				Confidence:  confidence,
				Evidence:    findingEvidence,
				Relations:   relatedEntities,
			}

			if !seenImpacted[impacted.ID] {
				seenImpacted[impacted.ID] = true
				result.Findings = append(result.Findings, finding)
			}
		}
	}

	if len(result.Findings) > 0 {
		result.Status = coreanalysis.StatusFindings
	}

	return result, nil
}

func evidenceKindForEntity(e *entity.Entity) coreanalysis.EvidenceKind {
	if e == nil {
		return coreanalysis.EvidenceKindRelation
	}
	switch e.Kind {
	case entity.KindFile:
		return coreanalysis.EvidenceKindFile
	case entity.KindPackage, entity.KindModule:
		return coreanalysis.EvidenceKindPackage
	case entity.KindConfig:
		return coreanalysis.EvidenceKindConfig
	default:
		return coreanalysis.EvidenceKindRelation
	}
}
