// Package orphan finds resources with no clear relationship or owner in the graph.
package orphan

import (
	"fmt"
	"time"

	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/graph"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
)

const (
	// AnalyzerName is the unique identifier for the Orphan analyzer.
	AnalyzerName = "orphan"

	// AnalyzerVersion is the version of this analyzer implementation.
	AnalyzerVersion = "1.0.0"

	// Finding Types.
	FindingTypeUnreferenced = "orphan_unreferenced"
	FindingTypeDisconnected = "orphan_disconnected"
	FindingTypeUnowned      = "orphan_unowned"
)

// Orphan analyzes entities in a RepairMap to identify unowned or unreferenced resources.
type Orphan struct {
	repairMap *repairmap.RepairMap
}

// New creates a new Orphan analyzer for the given RepairMap.
func New(rm *repairmap.RepairMap) (*Orphan, error) {
	if rm == nil {
		return nil, fmt.Errorf("orphan: repair map must not be nil")
	}

	return &Orphan{
		repairMap: rm,
	}, nil
}

// Name returns the analyzer identifier.
func (o *Orphan) Name() string {
	return AnalyzerName
}

// Version returns the analyzer version.
func (o *Orphan) Version() string {
	return AnalyzerVersion
}

// Analyze scans all entities and evaluates their structural relationships and ownership.
func (o *Orphan) Analyze() (*coreanalysis.AnalysisResult, error) {
	resultID := coreanalysis.ResultID(fmt.Sprintf("orphan-%d", time.Now().UTC().UnixNano()))

	result := &coreanalysis.AnalysisResult{
		ID:        resultID,
		Analyzer:  AnalyzerName,
		Version:   AnalyzerVersion,
		Timestamp: time.Now().UTC(),
		Status:    coreanalysis.StatusOK,
		Findings:  make([]coreanalysis.Finding, 0),
		Evidence:  make([]coreanalysis.DiagnosticEvidence, 0),
	}

	entities, err := o.repairMap.ListEntities(graph.Filter{})
	if err != nil {
		return nil, fmt.Errorf("orphan: failed to list entities: %w", err)
	}

	for _, e := range entities {
		// Check if entity is marked as a root/entrypoint entity
		if isRootEntity(e) {
			continue
		}

		// Query incoming and outgoing relations
		allRels, err := o.repairMap.AllRelationsFor(e.ID)
		if err != nil {
			return nil, fmt.Errorf("orphan: failed to query relations for %s: %w", e.ID, err)
		}

		incomingRels, err := o.repairMap.DirectDependents(e.ID, "")
		if err != nil {
			return nil, fmt.Errorf("orphan: failed to query incoming dependents for %s: %w", e.ID, err)
		}

		// Check for ownership relation
		hasOwner := false
		for _, r := range allRels {
			if r.Kind == relation.KindOwnedBy {
				hasOwner = true
				break
			}
		}

		// Case 1: Completely disconnected (0 relations at all)
		if len(allRels) == 0 {
			findingID := coreanalysis.FindingID(fmt.Sprintf("orphan-disconnected-%s", e.ID))
			result.Findings = append(result.Findings, coreanalysis.Finding{
				ID:          findingID,
				Severity:    coreanalysis.SeverityMedium,
				Type:        FindingTypeDisconnected,
				Subject:     e.ID,
				Description: fmt.Sprintf("Resource '%s' (%s) is completely disconnected (0 edges in dependency graph)", e.Name, e.Kind),
				Confidence:  coreanalysis.ConfidenceHigh,
				Evidence: []coreanalysis.DiagnosticEvidence{
					{
						Kind:     evidenceKindForEntity(e),
						SourceID: string(e.ID),
						Field:    "entity.kind",
						Value:    string(e.Kind),
					},
					{
						Kind:     coreanalysis.EvidenceKindRelation,
						SourceID: string(e.ID),
						Field:    "total_relations",
						Value:    0,
					},
				},
			})
			continue
		}

		// Case 2: Unreferenced (0 incoming relations and not owned)
		if len(incomingRels) == 0 && !hasOwner {
			findingID := coreanalysis.FindingID(fmt.Sprintf("orphan-unreferenced-%s", e.ID))
			result.Findings = append(result.Findings, coreanalysis.Finding{
				ID:          findingID,
				Severity:    coreanalysis.SeverityMedium,
				Type:        FindingTypeUnreferenced,
				Subject:     e.ID,
				Description: fmt.Sprintf("Resource '%s' (%s) is unreferenced and has no owner", e.Name, e.Kind),
				Confidence:  coreanalysis.ConfidenceHigh,
				Evidence: []coreanalysis.DiagnosticEvidence{
					{
						Kind:     evidenceKindForEntity(e),
						SourceID: string(e.ID),
						Field:    "entity.kind",
						Value:    string(e.Kind),
					},
					{
						Kind:     coreanalysis.EvidenceKindRelation,
						SourceID: string(e.ID),
						Field:    "incoming_relations_count",
						Value:    0,
					},
				},
			})
			continue
		}

		// Case 3: Used/active, but lacks an explicit owner relation (if required for key resources)
		if !hasOwner && requiresOwner(e) {
			findingID := coreanalysis.FindingID(fmt.Sprintf("orphan-unowned-%s", e.ID))
			result.Findings = append(result.Findings, coreanalysis.Finding{
				ID:          findingID,
				Severity:    coreanalysis.SeverityLow,
				Type:        FindingTypeUnowned,
				Subject:     e.ID,
				Description: fmt.Sprintf("Resource '%s' (%s) has dependencies but lacks an explicit owner relation", e.Name, e.Kind),
				Confidence:  coreanalysis.ConfidenceHigh,
				Evidence: []coreanalysis.DiagnosticEvidence{
					{
						Kind:     evidenceKindForEntity(e),
						SourceID: string(e.ID),
						Field:    "entity.kind",
						Value:    string(e.Kind),
					},
				},
			})
		}
	}

	if len(result.Findings) > 0 {
		result.Status = coreanalysis.StatusFindings
	}

	return result, nil
}

func isRootEntity(e *entity.Entity) bool {
	if val := getStringAttr(e.Attributes, "root"); val == "true" {
		return true
	}
	if val := getStringAttr(e.Attributes, "entrypoint"); val == "true" {
		return true
	}
	return false
}

func requiresOwner(e *entity.Entity) bool {
	// Critical infrastructure / services / key packages should track ownership
	return e.Kind == entity.KindService || e.Kind == "team_resource"
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

func getStringAttr(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	if v, ok := attrs[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
