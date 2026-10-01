// Package manualtrace finds repetitive manual workflows in event history and suggests automation.
package manualtrace

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
)

const (
	// ModuleName identifies this intelligence module.
	ModuleName = "manualtrace"

	// ModuleVersion is the version of this module.
	ModuleVersion = "1.0.0"

	// Defaults.
	DefaultMinLength      = 2
	DefaultMaxLength      = 6
	DefaultMinOccurrences = 2
	DefaultMaxGap         = 30 * time.Minute
)

// Step is one normalized action in a workflow sequence.
type Step struct {
	// Type is the event type (e.g. COMMAND_EXECUTED, CONFIG_CHANGED).
	Type event.Type `json:"type"`

	// Subject is the normalized event subject (command string, file path, etc.).
	Subject string `json:"subject"`
}

// Key returns a deterministic identity string for this step.
func (s Step) Key() string {
	return string(s.Type) + "|" + s.Subject
}

// String renders a human-readable step label.
func (s Step) String() string {
	switch s.Type {
	case event.TypeCommandExecuted:
		return "Command: " + s.Subject
	case event.TypeConfigChanged:
		return "Edit config: " + s.Subject
	case event.TypeFileChanged:
		return "Edit file: " + s.Subject
	default:
		return string(s.Type) + ": " + s.Subject
	}
}

// Pattern is a repeating workflow sequence detected in event history.
type Pattern struct {
	// Steps is the ordered sequence of workflow actions.
	Steps []Step `json:"steps"`

	// Occurrences is how many times this exact sequence was observed.
	Occurrences int `json:"occurrences"`

	// EventIDs lists the event IDs for each occurrence (evidence).
	// Outer index = occurrence; inner = steps within that occurrence.
	EventIDs [][]event.ID `json:"event_ids"`

	// Suggestion is a plain-language automation recommendation.
	Suggestion string `json:"suggestion"`
}

// SequenceKey returns a deterministic key for the step sequence.
func (p Pattern) SequenceKey() string {
	parts := make([]string, len(p.Steps))
	for i, s := range p.Steps {
		parts[i] = s.Key()
	}
	return strings.Join(parts, " -> ")
}

// Result holds ManualTrace analysis output.
type Result struct {
	// Module / Version identify the producer.
	Module  string `json:"module"`
	Version string `json:"version"`

	// Timestamp is when the analysis was performed.
	Timestamp time.Time `json:"timestamp"`

	// Patterns are repeating workflows sorted by occurrences (desc) then length (desc).
	Patterns []Pattern `json:"patterns"`

	// EventsScanned is the number of workflow-relevant events considered.
	EventsScanned int `json:"events_scanned"`

	// Sessions is the number of time-gap-separated activity sessions.
	Sessions int `json:"sessions"`
}

// HasPatterns reports whether any repetitive workflows were found.
func (r *Result) HasPatterns() bool {
	return r != nil && len(r.Patterns) > 0
}

// ManualTrace detects repetitive manual workflows from events.
type ManualTrace struct {
	minLength      int
	maxLength      int
	minOccurrences int
	maxGap         time.Duration
	types          map[event.Type]bool
}

// Option configures ManualTrace.
type Option func(*ManualTrace)

// WithMinLength sets the minimum sequence length (default 2).
func WithMinLength(n int) Option {
	return func(m *ManualTrace) {
		if n > 0 {
			m.minLength = n
		}
	}
}

// WithMaxLength sets the maximum sequence length (default 6).
func WithMaxLength(n int) Option {
	return func(m *ManualTrace) {
		if n > 0 {
			m.maxLength = n
		}
	}
}

// WithMinOccurrences sets how many times a sequence must repeat (default 2).
func WithMinOccurrences(n int) Option {
	return func(m *ManualTrace) {
		if n > 0 {
			m.minOccurrences = n
		}
	}
}

// WithMaxGap sets the maximum idle gap between events in one session (default 30m).
// Larger gaps split activity into separate sessions.
func WithMaxGap(d time.Duration) Option {
	return func(m *ManualTrace) {
		if d > 0 {
			m.maxGap = d
		}
	}
}

// WithEventTypes restricts which event types count as workflow steps.
// Default: COMMAND_EXECUTED, CONFIG_CHANGED, FILE_CHANGED.
func WithEventTypes(types ...event.Type) Option {
	return func(m *ManualTrace) {
		if len(types) == 0 {
			return
		}
		m.types = make(map[event.Type]bool, len(types))
		for _, t := range types {
			m.types[t] = true
		}
	}
}

// New creates a ManualTrace analyzer with optional configuration.
func New(opts ...Option) *ManualTrace {
	m := &ManualTrace{
		minLength:      DefaultMinLength,
		maxLength:      DefaultMaxLength,
		minOccurrences: DefaultMinOccurrences,
		maxGap:         DefaultMaxGap,
		types: map[event.Type]bool{
			event.TypeCommandExecuted: true,
			event.TypeConfigChanged:   true,
			event.TypeFileChanged:     true,
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	if m.maxLength < m.minLength {
		m.maxLength = m.minLength
	}
	return m
}

// Analyze detects repetitive sequences in a pre-fetched event list.
// Events are sorted chronologically; non-workflow types are ignored.
// This is the primary entry point when callers already queried the Event Store.
func (m *ManualTrace) Analyze(events []*event.Event) (*Result, error) {
	result := &Result{
		Module:    ModuleName,
		Version:   ModuleVersion,
		Timestamp: time.Now().UTC(),
		Patterns:  make([]Pattern, 0),
	}

	steps, ids := m.extractSteps(events)
	result.EventsScanned = len(steps)
	if len(steps) < m.minLength {
		return result, nil
	}

	sessions := m.splitSessions(events, steps, ids)
	result.Sessions = len(sessions)

	patterns := m.findPatterns(sessions)
	result.Patterns = patterns
	return result, nil
}

// AnalyzeStore queries the Event Store then runs Analyze.
func (m *ManualTrace) AnalyzeStore(ctx context.Context, store event.Store, q event.Query) (*Result, error) {
	if store == nil {
		return nil, errors.New(errors.CodeInvalidInput, "manualtrace: event store must not be nil")
	}
	// Force chronological order for sequence detection.
	q.Order = event.SortAsc
	events, err := store.Query(ctx, q)
	if err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "manualtrace: query events failed", err)
	}
	return m.Analyze(events)
}

type timedStep struct {
	step Step
	id   event.ID
	ts   time.Time
}

func (m *ManualTrace) extractSteps(events []*event.Event) ([]Step, []event.ID) {
	// Sort copy ascending by timestamp (ID tie-break for determinism).
	sorted := make([]*event.Event, 0, len(events))
	for _, e := range events {
		if e == nil {
			continue
		}
		sorted = append(sorted, e)
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].Timestamp.Equal(sorted[j].Timestamp) {
			return sorted[i].Timestamp.Before(sorted[j].Timestamp)
		}
		return sorted[i].ID < sorted[j].ID
	})

	steps := make([]Step, 0)
	ids := make([]event.ID, 0)
	for _, e := range sorted {
		if !m.types[e.Type] {
			continue
		}
		subj := normalizeSubject(e.Subject)
		if subj == "" {
			continue
		}
		steps = append(steps, Step{Type: e.Type, Subject: subj})
		ids = append(ids, e.ID)
	}
	return steps, ids
}

func (m *ManualTrace) splitSessions(allEvents []*event.Event, steps []Step, ids []event.ID) [][]timedStep {
	// Rebuild timed steps from sorted filtered events.
	timed := make([]timedStep, 0, len(steps))
	// Re-derive timestamps by matching ids against a map.
	byID := make(map[event.ID]*event.Event, len(allEvents))
	for _, e := range allEvents {
		if e != nil {
			byID[e.ID] = e
		}
	}
	for i, id := range ids {
		ts := time.Time{}
		if e, ok := byID[id]; ok {
			ts = e.Timestamp
		}
		timed = append(timed, timedStep{step: steps[i], id: id, ts: ts})
	}

	if len(timed) == 0 {
		return nil
	}

	sessions := make([][]timedStep, 0)
	current := []timedStep{timed[0]}
	for i := 1; i < len(timed); i++ {
		gap := timed[i].ts.Sub(timed[i-1].ts)
		if gap > m.maxGap {
			sessions = append(sessions, current)
			current = []timedStep{timed[i]}
			continue
		}
		current = append(current, timed[i])
	}
	sessions = append(sessions, current)
	return sessions
}

type patternAcc struct {
	steps []Step
	occ   [][]event.ID
}

func (m *ManualTrace) findPatterns(sessions [][]timedStep) []Pattern {
	acc := make(map[string]*patternAcc)

	for _, session := range sessions {
		n := len(session)
		for length := m.minLength; length <= m.maxLength && length <= n; length++ {
			for start := 0; start+length <= n; start++ {
				window := session[start : start+length]
				steps := make([]Step, length)
				ids := make([]event.ID, length)
				for i, ts := range window {
					steps[i] = ts.step
					ids[i] = ts.id
				}
				key := sequenceKey(steps)
				a, ok := acc[key]
				if !ok {
					a = &patternAcc{steps: steps, occ: make([][]event.ID, 0)}
					acc[key] = a
				}
				// Avoid counting overlapping duplicates within the same session window
				// that share the exact same event ID tuple.
				if !containsIDTuple(a.occ, ids) {
					a.occ = append(a.occ, ids)
				}
			}
		}
	}

	patterns := make([]Pattern, 0)
	for _, a := range acc {
		if len(a.occ) < m.minOccurrences {
			continue
		}
		patterns = append(patterns, Pattern{
			Steps:       a.steps,
			Occurrences: len(a.occ),
			EventIDs:    a.occ,
			Suggestion:  suggest(a.steps, len(a.occ)),
		})
	}

	// Prefer more frequent, then longer sequences; tie-break by key for determinism.
	sort.SliceStable(patterns, func(i, j int) bool {
		if patterns[i].Occurrences != patterns[j].Occurrences {
			return patterns[i].Occurrences > patterns[j].Occurrences
		}
		if len(patterns[i].Steps) != len(patterns[j].Steps) {
			return len(patterns[i].Steps) > len(patterns[j].Steps)
		}
		return patterns[i].SequenceKey() < patterns[j].SequenceKey()
	})

	// Drop patterns that are strict subsequences of a more/equally frequent longer pattern.
	patterns = suppressSubsequences(patterns)

	return patterns
}

func sequenceKey(steps []Step) string {
	parts := make([]string, len(steps))
	for i, s := range steps {
		parts[i] = s.Key()
	}
	return strings.Join(parts, "\x00")
}

func containsIDTuple(existing [][]event.ID, candidate []event.ID) bool {
	for _, row := range existing {
		if len(row) != len(candidate) {
			continue
		}
		same := true
		for i := range row {
			if row[i] != candidate[i] {
				same = false
				break
			}
		}
		if same {
			return true
		}
	}
	return false
}

func suppressSubsequences(patterns []Pattern) []Pattern {
	kept := make([]Pattern, 0, len(patterns))
	for i, p := range patterns {
		dominated := false
		pkey := p.SequenceKey()
		for j, other := range patterns {
			if i == j {
				continue
			}
			if len(other.Steps) <= len(p.Steps) {
				continue
			}
			if other.Occurrences < p.Occurrences {
				continue
			}
			if strings.Contains(other.SequenceKey(), pkey) || isSubsequence(p.Steps, other.Steps) {
				dominated = true
				break
			}
		}
		if !dominated {
			kept = append(kept, p)
		}
	}
	return kept
}

func isSubsequence(sub, full []Step) bool {
	if len(sub) == 0 || len(sub) > len(full) {
		return false
	}
	for start := 0; start+len(sub) <= len(full); start++ {
		match := true
		for i := range sub {
			if sub[i].Key() != full[start+i].Key() {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func suggest(steps []Step, occurrences int) string {
	labels := make([]string, len(steps))
	for i, s := range steps {
		labels[i] = s.String()
	}
	return fmt.Sprintf(
		"This sequence appears %d times:\n  %s\nConsider automating it.",
		occurrences,
		strings.Join(labels, " → "),
	)
}

func normalizeSubject(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
