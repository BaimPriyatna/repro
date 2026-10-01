// Package whybroken identifies likely failure causes from history, events, diffs, and the dependency graph.
//
// Findings are classified as confirmed_change, likely_contributor, possible_contributor, or unknown.
// Time proximity alone never yields confirmed_change or likely_contributor.
package whybroken

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/diff"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/graph"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
)

const (
	// AnalyzerName is the unique identifier for the WhyBroken analyzer.
	AnalyzerName = "whybroken"

	// AnalyzerVersion is the version of this analyzer implementation.
	AnalyzerVersion = "1.0.0"

	// Finding types.
	FindingTypeConfirmedChange     = "confirmed_change"
	FindingTypeLikelyContributor   = "likely_contributor"
	FindingTypePossibleContributor = "possible_contributor"
	FindingTypeUnknown             = "unknown"
)

// graphLink describes how a candidate entity relates to the failing subject.
type graphLink int

const (
	linkNone graphLink = iota
	linkSelf
	linkDirect
	linkTransitive
	linkInGraph // entity exists in graph but no subject was provided / no path found
)

// WhyBroken performs root-cause analysis by correlating Diff, Events, and RepairMap.
type WhyBroken struct {
	repairMap *repairmap.RepairMap
	before    *snapshot.Snapshot
	after     *snapshot.Snapshot
	events    []*event.Event
	subject   entity.ID
}

// Option configures the WhyBroken analyzer.
type Option func(*WhyBroken)

// WithSubject sets the optional failing component entity ID.
// When set, graph distance to the subject upgrades confidence classification.
func WithSubject(id entity.ID) Option {
	return func(w *WhyBroken) {
		w.subject = id
	}
}

// New creates a new WhyBroken analyzer.
//
// Required: non-nil RepairMap and both before/after snapshots (the history pair
// used for Diff). Events may be empty — callers pre-filter them for the relevant
// time window via the Event System.
func New(rm *repairmap.RepairMap, before, after *snapshot.Snapshot, events []*event.Event, opts ...Option) (*WhyBroken, error) {
	if rm == nil {
		return nil, fmt.Errorf("whybroken: repair map must not be nil: %w", analysis.ErrInvalidInput)
	}
	if before == nil || after == nil {
		return nil, fmt.Errorf("whybroken: before and after snapshots must not be nil: %w", analysis.ErrInvalidInput)
	}

	wb := &WhyBroken{
		repairMap: rm,
		before:    before,
		after:     after,
		events:    events,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(wb)
		}
	}
	return wb, nil
}

// Name returns the analyzer identifier.
func (w *WhyBroken) Name() string {
	return AnalyzerName
}

// Version returns the analyzer version.
func (w *WhyBroken) Version() string {
	return AnalyzerVersion
}

// Analyze executes root-cause analysis and produces an AnalysisResult.
func (w *WhyBroken) Analyze() (*coreanalysis.AnalysisResult, error) {
	resultID := coreanalysis.ResultID(fmt.Sprintf("whybroken-%s-%s", w.before.ID, w.after.ID))

	result := &coreanalysis.AnalysisResult{
		ID:        resultID,
		Analyzer:  AnalyzerName,
		Version:   AnalyzerVersion,
		Timestamp: time.Now().UTC(),
		Status:    coreanalysis.StatusOK,
		Findings:  make([]coreanalysis.Finding, 0),
		Evidence: []coreanalysis.DiagnosticEvidence{
			{
				Kind:      coreanalysis.EvidenceKindSnapshot,
				SourceID:  string(w.before.ID),
				Timestamp: w.before.Timestamp,
				Field:     "before",
			},
			{
				Kind:      coreanalysis.EvidenceKindSnapshot,
				SourceID:  string(w.after.ID),
				Timestamp: w.after.Timestamp,
				Field:     "after",
			},
		},
		RelatedSnapshots: []snapshot.ID{w.before.ID, w.after.ID},
		RelatedEvents:    make([]event.ID, 0, len(w.events)),
		RelatedEntities:  make([]entity.ID, 0),
	}

	for _, evt := range w.events {
		if evt != nil {
			result.RelatedEvents = append(result.RelatedEvents, evt.ID)
		}
	}

	// 1. Diff the snapshot pair (History → Diff input).
	diffResult, err := diff.Diff(w.before, w.after)
	if err != nil {
		return nil, fmt.Errorf("whybroken: diff failed: %w", err)
	}

	changes := diff.FilterChanges(diffResult.Changes,
		analysis.ChangeTypeAdded,
		analysis.ChangeTypeRemoved,
		analysis.ChangeTypeModified,
	)

	// 2. Build entity index from RepairMap.
	entities, err := w.repairMap.ListEntities(graph.Filter{})
	if err != nil {
		return nil, fmt.Errorf("whybroken: list entities failed: %w", err)
	}
	index := buildEntityIndex(entities)

	// Track which events were consumed by Diff-based findings.
	usedEvents := make(map[event.ID]bool)

	// 3. Classify Diff-backed candidates.
	findings := make([]coreanalysis.Finding, 0)
	seenSubjects := make(map[entity.ID]bool)

	for _, change := range changes {
		matched := matchChangeToEntity(change, index)
		if matched == nil {
			// Unmapped change: still emit as possible if we have no better signal,
			// but without a graph entity we treat path as an unknown-style subject.
			continue
		}

		corroborating := matchingEvents(matched, w.events)
		for _, evt := range corroborating {
			usedEvents[evt.ID] = true
		}

		link := w.classifyGraphLink(matched.ID)
		findingType, confidence, severity := classifyDiffCandidate(link, len(corroborating) > 0)

		finding := w.buildDiffFinding(matched, change, corroborating, findingType, confidence, severity, link)
		if !seenSubjects[matched.ID] {
			seenSubjects[matched.ID] = true
			findings = append(findings, finding)
			result.RelatedEntities = appendUniqueEntity(result.RelatedEntities, matched.ID)
		}
	}

	// 4. Classify leftover events (no Diff corroboration).
	for _, evt := range w.events {
		if evt == nil || usedEvents[evt.ID] {
			continue
		}

		matched := matchEventToEntity(evt, index)
		var finding coreanalysis.Finding
		if matched == nil {
			// Time-proximity alone → unknown. Never promote.
			finding = w.buildUnknownEventFinding(evt)
		} else {
			link := w.classifyGraphLink(matched.ID)
			if link == linkNone && w.subject != "" {
				// Entity in graph but not related to subject → still unknown
				finding = w.buildUnknownEventFinding(evt)
			} else if link == linkNone {
				// No subject: entity exists in graph but event has no Diff → possible at best
				// per plan: "event subject matches a graph entity without corroborating Diff"
				finding = w.buildEventOnlyFinding(matched, evt, FindingTypePossibleContributor,
					coreanalysis.ConfidenceLow, coreanalysis.SeverityLow, link)
				result.RelatedEntities = appendUniqueEntity(result.RelatedEntities, matched.ID)
			} else {
				// Graph-linked event without Diff → possible_contributor (never likely/confirmed).
				finding = w.buildEventOnlyFinding(matched, evt, FindingTypePossibleContributor,
					confidenceForEventOnly(link), severityForEventOnly(link), link)
				result.RelatedEntities = appendUniqueEntity(result.RelatedEntities, matched.ID)
			}
		}
		findings = append(findings, finding)
	}

	// 5. Deterministic ordering.
	sortFindings(findings)
	result.Findings = findings

	if len(findings) > 0 {
		result.Status = coreanalysis.StatusFindings
	}

	return result, nil
}

// Classification

func classifyDiffCandidate(link graphLink, hasEvent bool) (findingType string, confidence coreanalysis.Confidence, severity coreanalysis.Severity) {
	switch {
	case hasEvent && (link == linkSelf || link == linkDirect || link == linkTransitive):
		// Diff + Event + graph path to subject → confirmed.
		return FindingTypeConfirmedChange, coreanalysis.ConfidenceConfirmed, coreanalysis.SeverityCritical

	case hasEvent && link == linkInGraph:
		// Diff + Event + entity in graph, no subject → confirmed (strong multi-source evidence).
		return FindingTypeConfirmedChange, coreanalysis.ConfidenceConfirmed, coreanalysis.SeverityHigh

	case !hasEvent && (link == linkSelf || link == linkDirect):
		// Diff + direct dependency, no event → likely.
		return FindingTypeLikelyContributor, coreanalysis.ConfidenceHigh, coreanalysis.SeverityHigh

	case !hasEvent && link == linkInGraph:
		// Diff maps to graph entity, no subject / no event → likely (strong Diff+Graph).
		return FindingTypeLikelyContributor, coreanalysis.ConfidenceHigh, coreanalysis.SeverityMedium

	case !hasEvent && link == linkTransitive:
		// Diff + transitive only → possible.
		return FindingTypePossibleContributor, coreanalysis.ConfidenceMedium, coreanalysis.SeverityMedium

	case hasEvent && link == linkNone:
		// Diff + Event but entity not linked / not in useful graph relation.
		// Still have Diff evidence so not unknown; treat as possible.
		return FindingTypePossibleContributor, coreanalysis.ConfidenceLow, coreanalysis.SeverityLow

	default:
		// Diff change mapped to entity but no graph link and no event.
		return FindingTypePossibleContributor, coreanalysis.ConfidenceLow, coreanalysis.SeverityLow
	}
}

func confidenceForEventOnly(link graphLink) coreanalysis.Confidence {
	switch link {
	case linkSelf, linkDirect:
		return coreanalysis.ConfidenceMedium
	case linkTransitive:
		return coreanalysis.ConfidenceLow
	default:
		return coreanalysis.ConfidenceLow
	}
}

func severityForEventOnly(link graphLink) coreanalysis.Severity {
	switch link {
	case linkSelf, linkDirect:
		return coreanalysis.SeverityMedium
	default:
		return coreanalysis.SeverityLow
	}
}

func (w *WhyBroken) classifyGraphLink(candidateID entity.ID) graphLink {
	// Does the entity exist in the graph?
	if _, err := w.repairMap.GetEntity(candidateID); err != nil {
		return linkNone
	}

	if w.subject == "" {
		return linkInGraph
	}

	if candidateID == w.subject {
		return linkSelf
	}

	// Direct dependency or dependent?
	deps, _ := w.repairMap.DirectDependencies(w.subject, "")
	for _, d := range deps {
		if d.ID == candidateID {
			return linkDirect
		}
	}
	dependents, _ := w.repairMap.DirectDependents(w.subject, "")
	for _, d := range dependents {
		if d.ID == candidateID {
			return linkDirect
		}
	}

	// Transitive: path subject → candidate or candidate → subject (impact).
	pathFwd, _ := w.repairMap.ShortestPath(w.subject, candidateID, "")
	if len(pathFwd) > 2 {
		return linkTransitive
	}
	pathRev, _ := w.repairMap.ShortestPath(candidateID, w.subject, "")
	if len(pathRev) > 2 {
		return linkTransitive
	}

	// Also check ImpactSet of candidate (things affected if candidate changes).
	impacted, _ := w.repairMap.ImpactSet(candidateID, 50)
	for _, e := range impacted {
		if e.ID == w.subject {
			// Distance: if direct we already caught it; remaining is transitive.
			return linkTransitive
		}
	}

	return linkNone
}

// Finding builders

func (w *WhyBroken) buildDiffFinding(
	ent *entity.Entity,
	change analysis.Change,
	events []*event.Event,
	findingType string,
	confidence coreanalysis.Confidence,
	severity coreanalysis.Severity,
	link graphLink,
) coreanalysis.Finding {
	evidence := []coreanalysis.DiagnosticEvidence{
		{
			Kind:     coreanalysis.EvidenceKindDiff,
			SourceID: string(w.before.ID) + "->" + string(w.after.ID),
			Field:    change.Path,
			Value:    string(change.Type),
		},
		{
			Kind:     coreanalysis.EvidenceKindSnapshot,
			SourceID: string(w.before.ID),
			Field:    "before",
		},
		{
			Kind:     coreanalysis.EvidenceKindSnapshot,
			SourceID: string(w.after.ID),
			Field:    "after",
		},
	}

	if link != linkNone {
		evidence = append(evidence, coreanalysis.DiagnosticEvidence{
			Kind:     coreanalysis.EvidenceKindRelation,
			SourceID: string(ent.ID),
			Field:    "graph_link",
			Value:    linkLabel(link),
		})
	}

	related := []entity.ID{ent.ID}
	if w.subject != "" {
		related = append(related, w.subject)
		evidence = append(evidence, coreanalysis.DiagnosticEvidence{
			Kind:     coreanalysis.EvidenceKindRelation,
			SourceID: string(w.subject),
			Field:    "failing_subject",
			Value:    string(w.subject),
		})
	}

	for _, evt := range events {
		evidence = append(evidence, coreanalysis.DiagnosticEvidence{
			Kind:      coreanalysis.EvidenceKindEvent,
			SourceID:  string(evt.ID),
			Timestamp: evt.Timestamp,
			Field:     "subject",
			Value:     evt.Subject,
		})
	}

	name := ent.Name
	if name == "" {
		name = string(ent.ID)
	}

	return coreanalysis.Finding{
		ID:          coreanalysis.FindingID(fmt.Sprintf("whybroken-%s-%s", findingType, ent.ID)),
		Severity:    severity,
		Type:        findingType,
		Subject:     ent.ID,
		Description: describeDiffFinding(findingType, name, change, len(events) > 0, link),
		Confidence:  confidence,
		Evidence:    evidence,
		Relations:   related,
	}
}

func (w *WhyBroken) buildEventOnlyFinding(
	ent *entity.Entity,
	evt *event.Event,
	findingType string,
	confidence coreanalysis.Confidence,
	severity coreanalysis.Severity,
	link graphLink,
) coreanalysis.Finding {
	evidence := []coreanalysis.DiagnosticEvidence{
		{
			Kind:      coreanalysis.EvidenceKindEvent,
			SourceID:  string(evt.ID),
			Timestamp: evt.Timestamp,
			Field:     "subject",
			Value:     evt.Subject,
		},
		{
			Kind:     coreanalysis.EvidenceKindRelation,
			SourceID: string(ent.ID),
			Field:    "graph_link",
			Value:    linkLabel(link),
		},
	}

	related := []entity.ID{ent.ID}
	if w.subject != "" {
		related = append(related, w.subject)
	}

	name := ent.Name
	if name == "" {
		name = string(ent.ID)
	}

	return coreanalysis.Finding{
		ID:          coreanalysis.FindingID(fmt.Sprintf("whybroken-%s-%s-%s", findingType, ent.ID, evt.ID)),
		Severity:    severity,
		Type:        findingType,
		Subject:     ent.ID,
		Description: fmt.Sprintf("Event '%s' (%s) for '%s' lacks Diff corroboration; classified as %s (graph_link=%s)", evt.ID, evt.Type, name, findingType, linkLabel(link)),
		Confidence:  confidence,
		Evidence:    evidence,
		Relations:   related,
	}
}

func (w *WhyBroken) buildUnknownEventFinding(evt *event.Event) coreanalysis.Finding {
	return coreanalysis.Finding{
		ID:       coreanalysis.FindingID(fmt.Sprintf("whybroken-unknown-%s", evt.ID)),
		Severity: coreanalysis.SeverityInfo,
		Type:     FindingTypeUnknown,
		Subject:  entity.ID(evt.Subject),
		Description: fmt.Sprintf(
			"Event '%s' (%s, subject=%q) occurred in the time window but has no Diff or graph link; time proximity alone is not causal evidence",
			evt.ID, evt.Type, evt.Subject,
		),
		Confidence: coreanalysis.ConfidenceUnknown,
		Evidence: []coreanalysis.DiagnosticEvidence{
			{
				Kind:      coreanalysis.EvidenceKindEvent,
				SourceID:  string(evt.ID),
				Timestamp: evt.Timestamp,
				Field:     "subject",
				Value:     evt.Subject,
			},
		},
	}
}

func describeDiffFinding(findingType, name string, change analysis.Change, hasEvent bool, link graphLink) string {
	eventNote := "without corroborating event"
	if hasEvent {
		eventNote = "with corroborating event"
	}
	return fmt.Sprintf(
		"%s: '%s' changed (%s at %s) %s (graph_link=%s)",
		findingType, name, change.Type, change.Path, eventNote, linkLabel(link),
	)
}

func linkLabel(link graphLink) string {
	switch link {
	case linkSelf:
		return "self"
	case linkDirect:
		return "direct"
	case linkTransitive:
		return "transitive"
	case linkInGraph:
		return "in_graph"
	default:
		return "none"
	}
}

// Entity / event matching

type entityIndex struct {
	byID   map[entity.ID]*entity.Entity
	byName map[string]*entity.Entity
}

func buildEntityIndex(entities []*entity.Entity) *entityIndex {
	idx := &entityIndex{
		byID:   make(map[entity.ID]*entity.Entity, len(entities)),
		byName: make(map[string]*entity.Entity, len(entities)),
	}
	for _, e := range entities {
		if e == nil {
			continue
		}
		idx.byID[e.ID] = e
		if e.Name != "" {
			idx.byName[strings.ToLower(e.Name)] = e
		}
		// Also index by ID string as name for exact path segment matches.
		idx.byName[strings.ToLower(string(e.ID))] = e
	}
	return idx
}

func matchChangeToEntity(change analysis.Change, idx *entityIndex) *entity.Entity {
	path := change.Path
	if path == "" {
		return nil
	}

	// Exact ID / name match against full path.
	if e, ok := idx.byID[entity.ID(path)]; ok {
		return e
	}
	if e, ok := idx.byName[strings.ToLower(path)]; ok {
		return e
	}

	// Suffix / segment match: path "packages.react.version" → name "react".
	segments := strings.Split(path, ".")
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		if e, ok := idx.byID[entity.ID(seg)]; ok {
			return e
		}
		if e, ok := idx.byName[strings.ToLower(seg)]; ok {
			return e
		}
	}

	// Also try last path segment with common prefixes stripped conceptually
	// by checking whether any entity name is a path suffix.
	lowerPath := strings.ToLower(path)
	for name, e := range idx.byName {
		if name != "" && (strings.HasSuffix(lowerPath, "."+name) || strings.HasSuffix(lowerPath, "/"+name) || lowerPath == name) {
			return e
		}
	}

	return nil
}

func matchEventToEntity(evt *event.Event, idx *entityIndex) *entity.Entity {
	if evt == nil || evt.Subject == "" {
		return nil
	}
	subj := evt.Subject

	if e, ok := idx.byID[entity.ID(subj)]; ok {
		return e
	}
	if e, ok := idx.byName[strings.ToLower(subj)]; ok {
		return e
	}

	// Basename for file paths.
	base := subj
	if i := strings.LastIndexAny(subj, "/\\"); i >= 0 && i+1 < len(subj) {
		base = subj[i+1:]
	}
	if e, ok := idx.byName[strings.ToLower(base)]; ok {
		return e
	}
	if e, ok := idx.byID[entity.ID(base)]; ok {
		return e
	}

	lowerSubj := strings.ToLower(subj)
	for name, e := range idx.byName {
		if name != "" && (strings.HasSuffix(lowerSubj, name) || strings.Contains(lowerSubj, name)) {
			return e
		}
	}

	return nil
}

func matchingEvents(ent *entity.Entity, events []*event.Event) []*event.Event {
	if ent == nil {
		return nil
	}
	matched := make([]*event.Event, 0)
	nameLower := strings.ToLower(ent.Name)
	idLower := strings.ToLower(string(ent.ID))

	for _, evt := range events {
		if evt == nil {
			continue
		}
		subj := strings.ToLower(evt.Subject)
		if subj == idLower || subj == nameLower {
			matched = append(matched, evt)
			continue
		}
		if nameLower != "" && (strings.HasSuffix(subj, nameLower) || strings.Contains(subj, nameLower)) {
			matched = append(matched, evt)
			continue
		}
		if strings.HasSuffix(subj, idLower) || strings.Contains(subj, idLower) {
			matched = append(matched, evt)
		}
	}
	return matched
}

// Sorting / helpers

var typePriority = map[string]int{
	FindingTypeConfirmedChange:     0,
	FindingTypeLikelyContributor:   1,
	FindingTypePossibleContributor: 2,
	FindingTypeUnknown:             3,
}

func sortFindings(findings []coreanalysis.Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		pi, okI := typePriority[findings[i].Type]
		pj, okJ := typePriority[findings[j].Type]
		if !okI {
			pi = 99
		}
		if !okJ {
			pj = 99
		}
		if pi != pj {
			return pi < pj
		}
		if findings[i].Subject != findings[j].Subject {
			return findings[i].Subject < findings[j].Subject
		}
		return findings[i].ID < findings[j].ID
	})
}

func appendUniqueEntity(ids []entity.ID, id entity.ID) []entity.ID {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}
