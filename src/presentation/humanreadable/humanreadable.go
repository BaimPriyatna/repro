// Package humanreadable renders ExplainDiff output as plain-language diagnosis text.
package humanreadable

import (
	"fmt"
	"strings"
	"time"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/presentation/explaindiff"
)

const (
	// ModuleName identifies this presentation module.
	ModuleName = "humanreadable"

	// ModuleVersion is the version of this module.
	ModuleVersion = "1.0.0"
)

// Diagnosis is the plain-language rendering of an Explanation.
// It retains immutable references to the source explanation and WhyBroken
// result so every line remains traceable.
type Diagnosis struct {
	// ID uniquely identifies this diagnosis rendering.
	ID string `json:"id"`

	// Module / Version identify the producer.
	Module  string `json:"module"`
	Version string `json:"version"`

	// Timestamp is when the diagnosis was rendered.
	Timestamp time.Time `json:"timestamp"`

	// SourceExplanationID cites the ExplainDiff Explanation.
	SourceExplanationID string `json:"source_explanation_id"`

	// SourceResultID cites the original WhyBroken AnalysisResult.
	SourceResultID string `json:"source_result_id"`

	// Text is the plain-language diagnosis body.
	Text string `json:"text"`

	// FindingIDs lists the WhyBroken finding IDs covered (unchanged).
	FindingIDs []string `json:"finding_ids"`

	// EvidenceSourceIDs lists evidence source IDs cited (unchanged from Explanation).
	EvidenceSourceIDs []string `json:"evidence_source_ids"`
}

// HumanReadable renders an ExplainDiff Explanation as plain language.
type HumanReadable struct {
	explanation *explaindiff.Explanation
}

// New creates a HumanReadable renderer for the given Explanation.
func New(explanation *explaindiff.Explanation) (*HumanReadable, error) {
	if explanation == nil {
		return nil, errors.New(errors.CodeInvalidInput, "humanreadable: explanation must not be nil")
	}
	return &HumanReadable{explanation: explanation}, nil
}

// Render produces the plain-language Diagnosis without altering the underlying
// explanation or WhyBroken findings.
func (h *HumanReadable) Render() (*Diagnosis, error) {
	exp := h.explanation

	var b strings.Builder

	// Header
	b.WriteString(exp.Title)
	b.WriteString("\n\n")
	b.WriteString(exp.Summary)
	b.WriteString("\n")

	if exp.Context != "" {
		b.WriteString("\nContext:\n")
		b.WriteString(exp.Context)
		b.WriteString("\n")
	}

	// Observed
	b.WriteString("\nObserved:\n")
	if len(exp.Observed) == 0 {
		b.WriteString("- (none)\n")
	} else {
		for _, line := range exp.Observed {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Potential effects
	if len(exp.PotentialEffects) > 0 {
		b.WriteString("\nPotential effect:\n")
		for _, line := range exp.PotentialEffects {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Per-finding detail (preserves finding IDs verbatim)
	if len(exp.Findings) > 0 {
		b.WriteString("\nFindings:\n")
		for _, f := range exp.Findings {
			b.WriteString(fmt.Sprintf("- [%s] %s\n", f.FindingID, f.Narrative))
		}
	}

	// Evidence block
	b.WriteString("\nEvidence:\n")
	if len(exp.RelatedSnapshots) > 0 {
		for _, s := range exp.RelatedSnapshots {
			b.WriteString(fmt.Sprintf("- Snapshot %s\n", s))
		}
	}
	if len(exp.RelatedEvents) > 0 {
		for _, e := range exp.RelatedEvents {
			b.WriteString(fmt.Sprintf("- Event %s\n", e))
		}
	}
	seen := make(map[string]bool)
	for _, ref := range exp.Evidence {
		key := string(ref.Kind) + ":" + ref.SourceID
		if seen[key] {
			continue
		}
		seen[key] = true
		line := fmt.Sprintf("- %s %s", ref.Kind, ref.SourceID)
		if ref.Field != "" {
			line += " (" + ref.Field + ")"
		}
		if ref.Finding != "" {
			line += fmt.Sprintf(" ← finding %s", ref.Finding)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(exp.RelatedSnapshots) == 0 && len(exp.RelatedEvents) == 0 && len(exp.Evidence) == 0 {
		b.WriteString("- (no evidence citations)\n")
	}

	findingIDs := make([]string, 0, len(exp.Findings))
	for _, f := range exp.Findings {
		findingIDs = append(findingIDs, string(f.FindingID))
	}

	evidenceIDs := make([]string, 0, len(exp.Evidence))
	evSeen := make(map[string]bool)
	for _, ref := range exp.Evidence {
		if evSeen[ref.SourceID] {
			continue
		}
		evSeen[ref.SourceID] = true
		evidenceIDs = append(evidenceIDs, ref.SourceID)
	}

	return &Diagnosis{
		ID:                  fmt.Sprintf("humanreadable-%s", exp.ID),
		Module:              ModuleName,
		Version:             ModuleVersion,
		Timestamp:           time.Now().UTC(),
		SourceExplanationID: exp.ID,
		SourceResultID:      string(exp.SourceResultID),
		Text:                b.String(),
		FindingIDs:          findingIDs,
		EvidenceSourceIDs:   evidenceIDs,
	}, nil
}

// TracesToResult reports whether this diagnosis cites the given WhyBroken result ID.
func (d *Diagnosis) TracesToResult(resultID string) bool {
	return d != nil && d.SourceResultID == resultID
}

// TracesToExplanation reports whether this diagnosis cites the given Explanation ID.
func (d *Diagnosis) TracesToExplanation(explanationID string) bool {
	return d != nil && d.SourceExplanationID == explanationID
}
