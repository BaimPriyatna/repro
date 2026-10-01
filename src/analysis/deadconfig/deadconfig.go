// Package deadconfig finds configuration items that appear unused in the environment.
//
// Classification levels:
//   - definitely unused: config has zero references/incoming relations in the graph
//   - probably unused: config is only referenced by dead or orphan components
//   - unknown: config has dynamic, wildcard, or indeterminate usage patterns
package deadconfig

import (
	"fmt"
	"strings"
	"time"

	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/graph"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
)

const (
	// AnalyzerName is the unique identifier for the DeadConfig analyzer.
	AnalyzerName = "deadconfig"

	// AnalyzerVersion is the version of this analyzer implementation.
	AnalyzerVersion = "1.0.0"

	// Finding Types.
	FindingTypeDefinitelyUnused = "dead_config_definitely_unused"
	FindingTypeProbablyUnused   = "dead_config_probably_unused"
	FindingTypeUnknown          = "dead_config_unknown"
)

// DeadConfig analyzes configuration entities in a RepairMap to find unused items.
type DeadConfig struct {
	repairMap *repairmap.RepairMap
}

// New creates a new DeadConfig analyzer for the given RepairMap.
func New(rm *repairmap.RepairMap) (*DeadConfig, error) {
	if rm == nil {
		return nil, fmt.Errorf("deadconfig: repair map must not be nil")
	}

	return &DeadConfig{
		repairMap: rm,
	}, nil
}

// Name returns the analyzer identifier.
func (dc *DeadConfig) Name() string {
	return AnalyzerName
}

// Version returns the analyzer version.
func (dc *DeadConfig) Version() string {
	return AnalyzerVersion
}

// Analyze scans the graph for config entities and evaluates their usage.
func (dc *DeadConfig) Analyze() (*coreanalysis.AnalysisResult, error) {
	resultID := coreanalysis.ResultID(fmt.Sprintf("deadconfig-%d", time.Now().UTC().UnixNano()))

	result := &coreanalysis.AnalysisResult{
		ID:        resultID,
		Analyzer:  AnalyzerName,
		Version:   AnalyzerVersion,
		Timestamp: time.Now().UTC(),
		Status:    coreanalysis.StatusOK,
		Findings:  make([]coreanalysis.Finding, 0),
		Evidence:  make([]coreanalysis.DiagnosticEvidence, 0),
	}

	entities, err := dc.repairMap.ListEntities(graph.Filter{})
	if err != nil {
		return nil, fmt.Errorf("deadconfig: failed to list entities: %w", err)
	}

	for _, e := range entities {
		// Target config entities (KindConfig or metadata indicating config/env/secret)
		if !isConfigEntity(e) {
			continue
		}

		// Check if marked dynamic/indeterminate
		if isDynamicOrUnknown(e) {
			findingID := coreanalysis.FindingID(fmt.Sprintf("deadconfig-unknown-%s", e.ID))
			result.Findings = append(result.Findings, coreanalysis.Finding{
				ID:          findingID,
				Severity:    coreanalysis.SeverityInfo,
				Type:        FindingTypeUnknown,
				Subject:     e.ID,
				Description: fmt.Sprintf("Configuration '%s' (%s) has dynamic or indeterminate usage pattern", e.Name, e.Kind),
				Confidence:  coreanalysis.ConfidenceUnknown,
				Evidence: []coreanalysis.DiagnosticEvidence{
					{
						Kind:     coreanalysis.EvidenceKindConfig,
						SourceID: string(e.ID),
						Field:    "entity.kind",
						Value:    string(e.Kind),
					},
				},
			})
			continue
		}

		// Check incoming relations (who depends on or uses this config)
		dependents, err := dc.repairMap.DirectDependents(e.ID, "")
		if err != nil {
			return nil, fmt.Errorf("deadconfig: failed to query dependents for %s: %w", e.ID, err)
		}

		if len(dependents) == 0 {
			// Zero dependents -> definitely unused
			findingID := coreanalysis.FindingID(fmt.Sprintf("deadconfig-definitely-unused-%s", e.ID))
			result.Findings = append(result.Findings, coreanalysis.Finding{
				ID:          findingID,
				Severity:    coreanalysis.SeverityMedium,
				Type:        FindingTypeDefinitelyUnused,
				Subject:     e.ID,
				Description: fmt.Sprintf("Configuration '%s' (%s) is definitely unused (no incoming references in graph)", e.Name, e.Kind),
				Confidence:  coreanalysis.ConfidenceConfirmed,
				Evidence: []coreanalysis.DiagnosticEvidence{
					{
						Kind:     coreanalysis.EvidenceKindConfig,
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
		} else {
			// Has dependents: check if all dependents are themselves inactive or orphaned
			allDependentsDead := true
			dependentIDs := make([]entity.ID, 0, len(dependents))

			for _, dep := range dependents {
				dependentIDs = append(dependentIDs, dep.ID)
				// Check if the dependent itself has any incoming dependents or is active
				depOfDep, err := dc.repairMap.DirectDependents(dep.ID, "")
				if err == nil && len(depOfDep) > 0 {
					allDependentsDead = false
					break
				}
				// Also check attributes if marked deprecated/inactive
				if val := getStringAttr(dep.Attributes, "status"); val == "deprecated" {
					continue
				}
				if val := getStringAttr(dep.Attributes, "inactive"); val == "true" {
					continue
				}
				// If dep has 0 incoming dependents and is not a root service/app, it's considered dead
				if dep.Kind != entity.KindService && len(depOfDep) == 0 {
					continue
				}
				allDependentsDead = false
				break
			}

			if allDependentsDead {
				findingID := coreanalysis.FindingID(fmt.Sprintf("deadconfig-probably-unused-%s", e.ID))
				result.Findings = append(result.Findings, coreanalysis.Finding{
					ID:          findingID,
					Severity:    coreanalysis.SeverityLow,
					Type:        FindingTypeProbablyUnused,
					Subject:     e.ID,
					Description: fmt.Sprintf("Configuration '%s' (%s) is probably unused (only referenced by unreferenced/inactive components)", e.Name, e.Kind),
					Confidence:  coreanalysis.ConfidenceMedium,
					Evidence: []coreanalysis.DiagnosticEvidence{
						{
							Kind:     coreanalysis.EvidenceKindConfig,
							SourceID: string(e.ID),
							Field:    "entity.kind",
							Value:    string(e.Kind),
						},
					},
					Relations: dependentIDs,
				})
			}
		}
	}

	if len(result.Findings) > 0 {
		result.Status = coreanalysis.StatusFindings
	}

	return result, nil
}

func isConfigEntity(e *entity.Entity) bool {
	if e.Kind == entity.KindConfig {
		return true
	}
	if e.Kind == "env" || e.Kind == "environment" || e.Kind == "secret" || e.Kind == "feature_flag" {
		return true
	}
	// Check if name or attributes signifies config
	if val := getStringAttr(e.Attributes, "category"); strings.ToLower(val) == "config" {
		return true
	}
	return false
}

func isDynamicOrUnknown(e *entity.Entity) bool {
	val := getStringAttr(e.Attributes, "usage_pattern")
	if val == "dynamic" || val == "wildcard" || val == "unknown" {
		return true
	}
	if getStringAttr(e.Attributes, "dynamic") == "true" {
		return true
	}
	return false
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
