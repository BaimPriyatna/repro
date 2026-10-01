// Package depspy detects hidden and undeclared dependencies by comparing declared vs observed graphs.
//
// Finding types:
//   - hidden_dependency: used (observed) but not declared
//   - undeclared_relation: relation present in practice but absent from the declared model
//   - unused_declaration: declared but never observed as a dependency
package depspy

import (
	"fmt"
	"time"

	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/graph"
)

const (
	// AnalyzerName is the stable machine-readable identifier for Depspy.
	AnalyzerName = "depspy"
	// AnalyzerVersion is the current version of the Depspy implementation.
	AnalyzerVersion = "1.0.0"
)

// Depspy compares a declared dependency model against an observed dependency
// model and reports discrepancies as structured findings.
type Depspy struct {
	declared graph.Store
	observed graph.Store
}

// New creates a new Depspy analyzer.
//
// declared is the graph store representing explicitly declared dependencies.
// observed is the graph store representing actually observed dependencies.
//
// Both stores must be non-nil.
func New(declared, observed graph.Store) (*Depspy, error) {
	if declared == nil || observed == nil {
		return nil, fmt.Errorf("depspy: declared and observed stores must be non-nil")
	}
	return &Depspy{declared: declared, observed: observed}, nil
}

// Name returns the analyzer identifier.
func (d *Depspy) Name() string { return AnalyzerName }

// Version returns the analyzer version.
func (d *Depspy) Version() string { return AnalyzerVersion }

// Analyze performs the comparison and returns an AnalysisResult.
//
// The result contains three categories of findings:
// "hidden_dependency": entity observed but not declared.
// "undeclared_relation": relation observed but not declared.
// "unused_declaration": entity declared but never referenced as a target
// in any observed relation (i.e., declared but not depended upon by anyone).
func (d *Depspy) Analyze() (*coreanalysis.AnalysisResult, error) {
	now := time.Now().UTC()

	resultID := coreanalysis.ResultID(fmt.Sprintf("depspy-%d", now.UnixNano()))
	result := &coreanalysis.AnalysisResult{
		ID:        resultID,
		Analyzer:  AnalyzerName,
		Version:   AnalyzerVersion,
		Timestamp: now,
		Status:    coreanalysis.StatusOK,
		Findings:  make([]coreanalysis.Finding, 0),
		Evidence:  make([]coreanalysis.DiagnosticEvidence, 0),
	}

	var findings []coreanalysis.Finding

	// 1. Hidden dependencies: entities in observed but not in declared.
	hidden, err := d.findHiddenDependencies()
	if err != nil {
		return nil, fmt.Errorf("depspy: hidden dependency analysis: %w", err)
	}
	findings = append(findings, hidden...)

	// 2. Undeclared relations: relations in observed but not in declared.
	undeclared, err := d.findUndeclaredRelations()
	if err != nil {
		return nil, fmt.Errorf("depspy: undeclared relation analysis: %w", err)
	}
	findings = append(findings, undeclared...)

	// 3. Unused declarations: entities in declared never used as a dependency target
	// in any observed relation.
	unused, err := d.findUnusedDeclarations()
	if err != nil {
		return nil, fmt.Errorf("depspy: unused declaration analysis: %w", err)
	}
	findings = append(findings, unused...)

	result.Findings = findings
	if len(findings) > 0 {
		result.Status = coreanalysis.StatusFindings
	}
	return result, nil
}

// findHiddenDependencies returns findings for entities that are observed
// (actually used) but not declared in the declared model.
func (d *Depspy) findHiddenDependencies() ([]coreanalysis.Finding, error) {
	observedEntities, err := d.observed.ListEntities(graph.Filter{})
	if err != nil {
		return nil, err
	}

	findings := make([]coreanalysis.Finding, 0)
	for i, obs := range observedEntities {
		_, err := d.declared.GetEntity(obs.ID)
		if err == nil {
			// Exists in declared — not hidden.
			continue
		}
		// Not in declared graph → hidden dependency.
		finding := coreanalysis.Finding{
			ID:          coreanalysis.FindingID(fmt.Sprintf("depspy-hidden-%d", i)),
			Severity:    coreanalysis.SeverityHigh,
			Type:        "hidden_dependency",
			Subject:     obs.ID,
			Description: fmt.Sprintf("Entity %q (kind: %s) is used in practice but absent from the declared dependency model", obs.Name, obs.Kind),
			Confidence:  coreanalysis.ConfidenceHigh,
			Evidence: []coreanalysis.DiagnosticEvidence{
				{
					Kind:      coreanalysis.EvidenceKindRelation,
					SourceID:  string(obs.ID),
					Field:     "entity.kind",
					Value:     string(obs.Kind),
					Timestamp: time.Now().UTC(),
				},
			},
		}
		findings = append(findings, finding)
	}
	return findings, nil
}

// findUndeclaredRelations returns findings for relations that are observed
// but have no counterpart in the declared model.
func (d *Depspy) findUndeclaredRelations() ([]coreanalysis.Finding, error) {
	observedRelations, err := d.observed.ListRelations(graph.Filter{})
	if err != nil {
		return nil, err
	}

	findings := make([]coreanalysis.Finding, 0)
	for i, obs := range observedRelations {
		_, err := d.declared.GetRelation(obs.ID)
		if err == nil {
			// Exists in declared — not undeclared.
			continue
		}
		// Also check if a semantically equivalent relation exists (same from/to/kind).
		equiv, _ := d.declared.ListRelations(graph.Filter{
			FromID:       obs.FromID,
			ToID:         obs.ToID,
			RelationKind: obs.Kind,
		})
		if len(equiv) > 0 {
			// Semantically equivalent relation declared, just different ID.
			continue
		}

		finding := coreanalysis.Finding{
			ID:          coreanalysis.FindingID(fmt.Sprintf("depspy-undeclared-%d", i)),
			Severity:    coreanalysis.SeverityMedium,
			Type:        "undeclared_relation",
			Subject:     entity.ID(obs.FromID),
			Description: fmt.Sprintf("Relation %q (%s → %s, kind: %s) is observed at runtime but not declared", obs.ID, obs.FromID, obs.ToID, obs.Kind),
			Confidence:  coreanalysis.ConfidenceHigh,
			Evidence: []coreanalysis.DiagnosticEvidence{
				{
					Kind:      coreanalysis.EvidenceKindRelation,
					SourceID:  string(obs.ID),
					Field:     "relation.kind",
					Value:     string(obs.Kind),
					Timestamp: time.Now().UTC(),
				},
			},
		}
		findings = append(findings, finding)
	}
	return findings, nil
}

// findUnusedDeclarations returns findings for entities that are declared but
// never appear as the target (ToID) of any observed relation. This means they
// were declared as a dependency but never actually depended upon by any observed
// component.
func (d *Depspy) findUnusedDeclarations() ([]coreanalysis.Finding, error) {
	declaredEntities, err := d.declared.ListEntities(graph.Filter{})
	if err != nil {
		return nil, err
	}

	findings := make([]coreanalysis.Finding, 0)
	for i, decl := range declaredEntities {
		// Check if anything in the observed graph depends on this entity.
		observedAsTarget, err := d.observed.ListRelations(graph.Filter{ToID: decl.ID})
		if err != nil {
			return nil, err
		}
		if len(observedAsTarget) > 0 {
			// Someone depends on it — it is used.
			continue
		}
		// Also check if it appears as a FromID in observed (it actively depends on others).
		// An entity that only has outgoing edges but no incoming edges can still be a root.
		// We only flag it if it is also absent from observed entirely (truly unused).
		_, getErr := d.observed.GetEntity(decl.ID)
		if getErr == nil {
			// Present in observed — even if not a target, it's not unused.
			continue
		}

		finding := coreanalysis.Finding{
			ID:          coreanalysis.FindingID(fmt.Sprintf("depspy-unused-%d", i)),
			Severity:    coreanalysis.SeverityLow,
			Type:        "unused_declaration",
			Subject:     decl.ID,
			Description: fmt.Sprintf("Entity %q (kind: %s) is declared as a dependency but never observed as a dependency target", decl.Name, decl.Kind),
			Confidence:  coreanalysis.ConfidenceMedium,
			Evidence: []coreanalysis.DiagnosticEvidence{
				{
					Kind:      coreanalysis.EvidenceKindRelation,
					SourceID:  string(decl.ID),
					Field:     "entity.kind",
					Value:     string(decl.Kind),
					Timestamp: time.Now().UTC(),
				},
			},
		}
		findings = append(findings, finding)
	}
	return findings, nil
}
