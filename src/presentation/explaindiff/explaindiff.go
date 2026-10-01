// Package explaindiff turns WhyBroken findings into structured explanations that cite evidence.
package explaindiff

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

const (
	// ModuleName identifies this presentation module.
	ModuleName = "explaindiff"

	// ModuleVersion is the version of this module.
	ModuleVersion = "1.0.0"

	// ExpectedAnalyzer is the analyzer this module is scoped to explain.
	ExpectedAnalyzer = "whybroken"
)

// EvidenceRef is a citation back to a concrete diagnostic artifact.
// Every explanation block must retain at least one EvidenceRef so output
// traces to WhyBroken evidence.
type EvidenceRef struct {
	Kind     coreanalysis.EvidenceKind `json:"kind"`
	SourceID string                    `json:"source_id"`
	Field    string                    `json:"field,omitempty"`
	Finding  coreanalysis.FindingID    `json:"finding_id,omitempty"`
}

// ExplainedFinding is one WhyBroken finding rendered as narrative, still
// bound to its original evidence and finding ID.
type ExplainedFinding struct {
	FindingID  coreanalysis.FindingID  `json:"finding_id"`
	Type       string                  `json:"type"`
	Subject    string                  `json:"subject"`
	Confidence coreanalysis.Confidence `json:"confidence"`
	Severity   coreanalysis.Severity   `json:"severity"`
	Narrative  string                  `json:"narrative"`
	Evidence   []EvidenceRef           `json:"evidence"`
}

// Explanation is the structured human-readable output of ExplainDiff.
type Explanation struct {
	// ID uniquely identifies this explanation.
	ID string `json:"id"`

	// Module / Version identify the producer.
	Module  string `json:"module"`
	Version string `json:"version"`

	// Timestamp is when the explanation was produced.
	Timestamp time.Time `json:"timestamp"`

	// SourceResultID is the WhyBroken AnalysisResult ID this explanation cites.
	SourceResultID coreanalysis.ResultID `json:"source_result_id"`

	// SourceAnalyzer echoes the analyzer name.
	SourceAnalyzer string `json:"source_analyzer"`

	// Title is a short headline for the explanation.
	Title string `json:"title"`

	// Summary is a one-paragraph overview.
	Summary string `json:"summary"`

	// Context is optional caller-supplied situational context.
	Context string `json:"context,omitempty"`

	// Observed lists concrete changes / findings observed.
	Observed []string `json:"observed"`

	// PotentialEffects describes possible consequences (non-causal claims).
	PotentialEffects []string `json:"potential_effects"`

	// Findings holds per-finding narratives.
	Findings []ExplainedFinding `json:"findings"`

	// Evidence aggregates all citations used in this explanation.
	Evidence []EvidenceRef `json:"evidence"`

	// RelatedSnapshots / RelatedEvents mirror the source result for traceability.
	RelatedSnapshots []snapshot.ID `json:"related_snapshots,omitempty"`
	RelatedEvents    []event.ID    `json:"related_events,omitempty"`
}

// ExplainDiff produces structured explanations from a WhyBroken AnalysisResult.
type ExplainDiff struct {
	result  *coreanalysis.AnalysisResult
	diff    *analysis.DiffResult
	context string
}

// Option configures ExplainDiff.
type Option func(*ExplainDiff)

// WithDiff supplies an optional DiffResult for richer "Observed" narration.
// ExplainDiff never re-diffs; it only summarizes the provided DiffResult.
func WithDiff(d *analysis.DiffResult) Option {
	return func(e *ExplainDiff) {
		e.diff = d
	}
}

// WithContext attaches free-text situational context.
func WithContext(ctx string) Option {
	return func(e *ExplainDiff) {
		e.context = strings.TrimSpace(ctx)
	}
}

// New creates an ExplainDiff for a WhyBroken AnalysisResult.
func New(result *coreanalysis.AnalysisResult, opts ...Option) (*ExplainDiff, error) {
	if result == nil {
		return nil, errors.New(errors.CodeInvalidInput, "explaindiff: analysis result must not be nil")
	}
	if result.Analyzer != ExpectedAnalyzer {
		return nil, errors.New(
			errors.CodeInvalidInput,
			fmt.Sprintf("explaindiff: expected analyzer %q, got %q (scoped to WhyBroken)", ExpectedAnalyzer, result.Analyzer),
		)
	}

	ed := &ExplainDiff{result: result}
	for _, opt := range opts {
		if opt != nil {
			opt(ed)
		}
	}
	return ed, nil
}

// Explain produces the structured Explanation.
func (e *ExplainDiff) Explain() (*Explanation, error) {
	r := e.result

	exp := &Explanation{
		ID:               fmt.Sprintf("explaindiff-%s", r.ID),
		Module:           ModuleName,
		Version:          ModuleVersion,
		Timestamp:        time.Now().UTC(),
		SourceResultID:   r.ID,
		SourceAnalyzer:   r.Analyzer,
		Context:          e.context,
		Observed:         make([]string, 0),
		PotentialEffects: make([]string, 0),
		Findings:         make([]ExplainedFinding, 0, len(r.Findings)),
		Evidence:         make([]EvidenceRef, 0),
		RelatedSnapshots: append([]snapshot.ID(nil), r.RelatedSnapshots...),
		RelatedEvents:    append([]event.ID(nil), r.RelatedEvents...),
	}

	// Result-level evidence citations.
	for _, ev := range r.Evidence {
		exp.Evidence = append(exp.Evidence, EvidenceRef{
			Kind:     ev.Kind,
			SourceID: ev.SourceID,
			Field:    ev.Field,
		})
	}

	// Snapshot pair framing.
	snapA, snapB := snapshotPair(r)
	if snapA != "" && snapB != "" {
		exp.Title = "Environment changes between snapshots"
		exp.Summary = fmt.Sprintf(
			"The environment changed between Snapshot %s and Snapshot %s. WhyBroken reported %d candidate cause(s).",
			snapA, snapB, len(r.Findings),
		)
	} else {
		exp.Title = "Root-cause diagnostic explanation"
		exp.Summary = fmt.Sprintf("WhyBroken reported %d candidate cause(s).", len(r.Findings))
	}
	if e.context != "" {
		exp.Summary += " Context: " + e.context
	}

	// Optional Diff summary enrichment (does not replace finding evidence).
	if e.diff != nil && e.diff.HasChanges() {
		s := e.diff.Summary
		exp.Observed = append(exp.Observed, fmt.Sprintf(
			"Snapshot Diff: %d added, %d removed, %d modified",
			s.Added, s.Removed, s.Modified,
		))
		exp.Evidence = append(exp.Evidence, EvidenceRef{
			Kind:     coreanalysis.EvidenceKindDiff,
			SourceID: string(e.diff.SnapshotA) + "->" + string(e.diff.SnapshotB),
			Field:    "diff_summary",
		})
	}

	// Per-finding narratives — preserve finding IDs and evidence (no re-diagnosis).
	for _, f := range r.Findings {
		explained := explainFinding(f)
		exp.Findings = append(exp.Findings, explained)
		exp.Observed = append(exp.Observed, observedLine(f))
		if effect := potentialEffect(f); effect != "" {
			exp.PotentialEffects = append(exp.PotentialEffects, effect)
		}
		exp.Evidence = append(exp.Evidence, explained.Evidence...)
	}

	if len(r.Findings) == 0 {
		exp.Observed = append(exp.Observed, "No candidate causes were identified.")
		exp.Summary = strings.TrimSpace(exp.Summary + " No findings to explain.")
	}

	// Deterministic ordering of potential effects / evidence by SourceID.
	sort.Strings(exp.PotentialEffects)
	sortEvidence(exp.Evidence)

	return exp, nil
}

func explainFinding(f coreanalysis.Finding) ExplainedFinding {
	refs := make([]EvidenceRef, 0, len(f.Evidence))
	for _, ev := range f.Evidence {
		refs = append(refs, EvidenceRef{
			Kind:     ev.Kind,
			SourceID: ev.SourceID,
			Field:    ev.Field,
			Finding:  f.ID,
		})
	}

	narrative := fmt.Sprintf(
		"%s (%s): %s [confidence=%s, severity=%s]",
		labelForType(f.Type), f.Subject, f.Description, f.Confidence, f.Severity,
	)

	return ExplainedFinding{
		FindingID:  f.ID,
		Type:       f.Type,
		Subject:    string(f.Subject),
		Confidence: f.Confidence,
		Severity:   f.Severity,
		Narrative:  narrative,
		Evidence:   refs,
	}
}

func observedLine(f coreanalysis.Finding) string {
	return fmt.Sprintf("%s on %s — %s", labelForType(f.Type), f.Subject, f.Description)
}

func potentialEffect(f coreanalysis.Finding) string {
	switch f.Type {
	case "confirmed_change":
		return fmt.Sprintf("Confirmed change in %s is a strong candidate for the observed failure.", f.Subject)
	case "likely_contributor":
		return fmt.Sprintf("%s likely contributes to the failure based on Diff and dependency evidence.", f.Subject)
	case "possible_contributor":
		return fmt.Sprintf("%s may contribute; evidence is partial (transitive link or uncorroborated event).", f.Subject)
	case "unknown":
		return fmt.Sprintf("Event related to %s remains unexplained; time proximity alone is not causal.", f.Subject)
	default:
		return fmt.Sprintf("Change involving %s may affect dependent components.", f.Subject)
	}
}

func labelForType(t string) string {
	switch t {
	case "confirmed_change":
		return "Confirmed change"
	case "likely_contributor":
		return "Likely contributor"
	case "possible_contributor":
		return "Possible contributor"
	case "unknown":
		return "Unknown"
	default:
		return t
	}
}

func snapshotPair(r *coreanalysis.AnalysisResult) (string, string) {
	if len(r.RelatedSnapshots) >= 2 {
		return string(r.RelatedSnapshots[0]), string(r.RelatedSnapshots[1])
	}
	var a, b string
	for _, ev := range r.Evidence {
		if ev.Kind != coreanalysis.EvidenceKindSnapshot {
			continue
		}
		switch ev.Field {
		case "before":
			a = ev.SourceID
		case "after":
			b = ev.SourceID
		}
	}
	return a, b
}

func sortEvidence(refs []EvidenceRef) {
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Kind != refs[j].Kind {
			return refs[i].Kind < refs[j].Kind
		}
		if refs[i].SourceID != refs[j].SourceID {
			return refs[i].SourceID < refs[j].SourceID
		}
		return string(refs[i].Finding) < string(refs[j].Finding)
	})
}

// HasEvidence reports whether the explanation cites at least one evidence ref.
func (e *Explanation) HasEvidence() bool {
	return e != nil && len(e.Evidence) > 0
}

// FindingIDs returns the ordered list of source finding IDs explained.
func (e *Explanation) FindingIDs() []coreanalysis.FindingID {
	if e == nil {
		return nil
	}
	ids := make([]coreanalysis.FindingID, len(e.Findings))
	for i, f := range e.Findings {
		ids[i] = f.FindingID
	}
	return ids
}
