// Package analysis defines AnalysisResult and DiagnosticEvidence domain types.
//
// Confidence and severity are separate axes: severity is importance, confidence is certainty.
package analysis

import (
	"time"

	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// Confidence

// Confidence expresses how certain an analysis module is about a finding.
// Use explicit categories — do not convert these into fake numerical
// probabilities unless a validated statistical model exists.
type Confidence string

const (
	// ConfidenceConfirmed means the evidence directly proves the finding.
	ConfidenceConfirmed Confidence = "confirmed"

	// ConfidenceHigh means strong evidence supports the finding.
	ConfidenceHigh Confidence = "high"

	// ConfidenceMedium means moderate evidence supports the finding.
	ConfidenceMedium Confidence = "medium"

	// ConfidenceLow means weak or indirect evidence supports the finding.
	ConfidenceLow Confidence = "low"

	// ConfidenceUnknown means the analysis could not determine confidence.
	ConfidenceUnknown Confidence = "unknown"
)

// Severity

// Severity expresses the importance of a finding.
// Severity is independent of Confidence: a finding can be
// high-severity but low-confidence, or low-severity but confirmed.
type Severity string

const (
	// SeverityCritical means the finding likely causes or caused a failure.
	SeverityCritical Severity = "critical"

	// SeverityHigh means the finding is important and should be investigated.
	SeverityHigh Severity = "high"

	// SeverityMedium means the finding is notable but not immediately critical.
	SeverityMedium Severity = "medium"

	// SeverityLow means the finding is informational.
	SeverityLow Severity = "low"

	// SeverityInfo is for purely informational findings without a risk rating.
	SeverityInfo Severity = "info"
)

// ResultStatus

// ResultStatus describes the overall outcome of an analysis run.
type ResultStatus string

const (
	// StatusOK means the analysis completed normally with no findings.
	StatusOK ResultStatus = "ok"

	// StatusFindings means the analysis completed and produced one or more findings.
	StatusFindings ResultStatus = "findings"

	// StatusError means the analysis did not complete due to an internal error.
	StatusError ResultStatus = "error"

	// StatusPartial means the analysis completed but some inputs were unavailable.
	StatusPartial ResultStatus = "partial"
)

// EvidenceKind

// EvidenceKind classifies what type of artifact an Evidence item references.
type EvidenceKind string

const (
	EvidenceKindSnapshot  EvidenceKind = "snapshot"
	EvidenceKindEvent     EvidenceKind = "event"
	EvidenceKindDiff      EvidenceKind = "diff"
	EvidenceKindFile      EvidenceKind = "file"
	EvidenceKindPackage   EvidenceKind = "package"
	EvidenceKindConfig    EvidenceKind = "config"
	EvidenceKindRelation  EvidenceKind = "relation"
	EvidenceKindCommand   EvidenceKind = "command"
	EvidenceKindGitCommit EvidenceKind = "git_commit"
)

// DiagnosticEvidence

// DiagnosticEvidence is a reference to a concrete artifact that supports a finding.
// Items are references only — they do not embed the full artifact payload.
type DiagnosticEvidence struct {
	// Kind classifies what type of artifact this evidence references.
	Kind EvidenceKind `json:"kind"`

	// SourceID is the stable identifier of the referenced artifact.
	SourceID string `json:"source_id"`

	// Timestamp is the time at which the evidence was recorded, if known.
	Timestamp time.Time `json:"timestamp,omitempty"`

	// Field optionally names the specific field within the artifact that is
	// relevant (e.g. "runtime.version", "env.NODE_ENV").
	Field string `json:"field,omitempty"`

	// Value optionally records the value of the field at the time of capture.
	Value any `json:"value,omitempty"`
}

// Finding

// FindingID is a stable, unique identifier for a single finding within a result.
type FindingID string

// Finding is a single diagnostic observation produced by an analysis module.
type Finding struct {
	// ID is a stable identifier for this finding within the analysis result.
	ID FindingID `json:"id"`

	// Severity expresses how important this finding is.
	Severity Severity `json:"severity"`

	// Type is a machine-readable classifier for the finding.
	Type string `json:"type"`

	// Subject identifies the primary entity or resource this finding is about.
	Subject entity.ID `json:"subject"`

	// Description is a concise human-readable explanation of the finding.
	Description string `json:"description"`

	// Confidence expresses how certain the analysis module is about this finding.
	Confidence Confidence `json:"confidence"`

	// Evidence is the list of artifact references that support this finding.
	// A finding without evidence is not valid.
	Evidence []DiagnosticEvidence `json:"evidence"`

	// Relations lists the entity IDs that are structurally related to this
	// finding (e.g. downstream dependents, affected configs).
	Relations []entity.ID `json:"relations,omitempty"`
}

// HasEvidence reports whether the finding has at least one piece of supporting
// evidence. A finding without evidence is invalid.
func (f *Finding) HasEvidence() bool {
	return len(f.Evidence) > 0
}

// Result

// ResultID is a stable, unique identifier for an analysis result.
type ResultID string

// Result is the common output type for analysis modules.
type Result struct {
	// ID is the unique identifier for this result.
	ID ResultID `json:"id"`

	// Analyzer is the machine-readable name of the module that produced this
	// result (e.g. "beforeafter", "drift", "whybroken").
	Analyzer string `json:"analyzer"`

	// Version is the version of the analyzer that produced this result.
	// Used to detect schema drift across module upgrades.
	Version string `json:"version"`

	// Timestamp is the wall-clock time when the analysis was performed.
	Timestamp time.Time `json:"timestamp"`

	// Status describes the overall outcome of the analysis run.
	Status ResultStatus `json:"status"`

	// Findings is the list of diagnostic observations. May be empty (StatusOK).
	Findings []Finding `json:"findings"`

	// Evidence is result-level evidence shared across all findings.
	// Finding-level evidence is stored on each Finding. This field holds
	// evidence that applies to the overall analysis context.
	Evidence []DiagnosticEvidence `json:"evidence"`

	// RelatedSnapshots lists snapshot IDs that were consumed as input.
	RelatedSnapshots []snapshot.ID `json:"related_snapshots,omitempty"`

	// RelatedEvents lists event IDs that were consumed as input.
	RelatedEvents []event.ID `json:"related_events,omitempty"`

	// RelatedEntities lists entity IDs relevant to this result.
	RelatedEntities []entity.ID `json:"related_entities,omitempty"`
}

// FindingCount returns the number of findings in the result.
func (r *Result) FindingCount() int {
	return len(r.Findings)
}

// IsError reports whether the analysis terminated with an error.
func (r *Result) IsError() bool {
	return r.Status == StatusError
}

// HasFindings reports whether the result contains at least one finding.
func (r *Result) HasFindings() bool {
	return len(r.Findings) > 0
}

// AnalysisResult is a backward-compatibility alias for Result.
//
//nolint:revive
type AnalysisResult = Result //nolint:revive // stutters: kept for external callers
