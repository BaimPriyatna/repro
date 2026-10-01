package manualtrace_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/intelligence/manualtrace"
)

func mustEvent(t *testing.T, id event.ID, typ event.Type, subject string, ts time.Time) *event.Event {
	t.Helper()
	evt, err := event.New(typ, event.SourceUser, subject,
		event.WithID(id),
		event.WithTimestamp(ts),
	)
	if err != nil {
		t.Fatalf("event.New: %v", err)
	}
	return evt
}

func TestAnalyze_NoEvents(t *testing.T) {
	mt := manualtrace.New()
	result, err := mt.Analyze(nil)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.HasPatterns() {
		t.Error("expected no patterns")
	}
	if result.Module != manualtrace.ModuleName {
		t.Errorf("Module = %q", result.Module)
	}
}

func TestAnalyze_DetectsRepeatingCommandSequence(t *testing.T) {
	t0 := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

	// Same sequence twice: build → test → deploy
	events := []*event.Event{
		mustEvent(t, "e1", event.TypeCommandExecuted, "make build", t0),
		mustEvent(t, "e2", event.TypeCommandExecuted, "make test", t0.Add(1*time.Minute)),
		mustEvent(t, "e3", event.TypeCommandExecuted, "make deploy", t0.Add(2*time.Minute)),
		// gap within session
		mustEvent(t, "e4", event.TypeCommandExecuted, "make build", t0.Add(5*time.Minute)),
		mustEvent(t, "e5", event.TypeCommandExecuted, "make test", t0.Add(6*time.Minute)),
		mustEvent(t, "e6", event.TypeCommandExecuted, "make deploy", t0.Add(7*time.Minute)),
	}

	mt := manualtrace.New(
		manualtrace.WithMinLength(3),
		manualtrace.WithMinOccurrences(2),
	)
	result, err := mt.Analyze(events)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.HasPatterns() {
		t.Fatal("expected repeating pattern")
	}

	found := false
	for _, p := range result.Patterns {
		if len(p.Steps) == 3 &&
			p.Steps[0].Subject == "make build" &&
			p.Steps[1].Subject == "make test" &&
			p.Steps[2].Subject == "make deploy" {
			found = true
			if p.Occurrences < 2 {
				t.Errorf("Occurrences = %d, want >= 2", p.Occurrences)
			}
			if !strings.Contains(p.Suggestion, "Consider automating") {
				t.Errorf("Suggestion = %q", p.Suggestion)
			}
			if len(p.EventIDs) < 2 {
				t.Error("EventIDs must provide evidence for each occurrence")
			}
		}
	}
	if !found {
		t.Fatalf("expected build→test→deploy pattern, got %+v", result.Patterns)
	}
}

func TestAnalyze_MixedWorkflowSteps(t *testing.T) {
	t0 := time.Date(2026, 9, 30, 11, 0, 0, 0, time.UTC)

	// Command A, Edit config, Command B — repeated
	seq := func(offset time.Duration, prefix string) []*event.Event {
		base := t0.Add(offset)
		return []*event.Event{
			mustEvent(t, event.ID(prefix+"1"), event.TypeCommandExecuted, "npm install", base),
			mustEvent(t, event.ID(prefix+"2"), event.TypeConfigChanged, "package.json", base.Add(time.Minute)),
			mustEvent(t, event.ID(prefix+"3"), event.TypeCommandExecuted, "npm start", base.Add(2*time.Minute)),
		}
	}

	events := append(seq(0, "a"), seq(10*time.Minute, "b")...)

	mt := manualtrace.New(manualtrace.WithMinLength(3), manualtrace.WithMinOccurrences(2))
	result, err := mt.Analyze(events)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.HasPatterns() {
		t.Fatal("expected mixed workflow pattern")
	}

	found := false
	for _, p := range result.Patterns {
		if len(p.Steps) == 3 &&
			p.Steps[0].Type == event.TypeCommandExecuted &&
			p.Steps[1].Type == event.TypeConfigChanged &&
			p.Steps[2].Type == event.TypeCommandExecuted {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected command→config→command pattern, got %+v", result.Patterns)
	}
}

func TestAnalyze_SessionGapSplitsSequences(t *testing.T) {
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

	events := []*event.Event{
		mustEvent(t, "g1", event.TypeCommandExecuted, "cmd-a", t0),
		mustEvent(t, "g2", event.TypeCommandExecuted, "cmd-b", t0.Add(time.Minute)),
		// Large gap → new session; sequence does not bridge the gap
		mustEvent(t, "g3", event.TypeCommandExecuted, "cmd-a", t0.Add(2*time.Hour)),
		mustEvent(t, "g4", event.TypeCommandExecuted, "cmd-b", t0.Add(2*time.Hour+time.Minute)),
	}

	mt := manualtrace.New(
		manualtrace.WithMinLength(2),
		manualtrace.WithMinOccurrences(2),
		manualtrace.WithMaxGap(30*time.Minute),
	)
	result, err := mt.Analyze(events)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Sessions != 2 {
		t.Errorf("Sessions = %d, want 2", result.Sessions)
	}
	// Pattern still detected across sessions (same sequence in each)
	if !result.HasPatterns() {
		t.Fatal("expected pattern across two sessions")
	}
}

func TestAnalyze_IgnoresNonWorkflowEvents(t *testing.T) {
	t0 := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)

	events := []*event.Event{
		mustEvent(t, "n1", event.TypeCommandExecuted, "echo hi", t0),
		mustEvent(t, "n2", event.TypeSnapshotCreated, "snap-1", t0.Add(time.Minute)),
		mustEvent(t, "n3", event.TypeCommandExecuted, "echo hi", t0.Add(2*time.Minute)),
		mustEvent(t, "n4", event.TypeGitCommit, "abc123", t0.Add(3*time.Minute)),
	}

	mt := manualtrace.New(
		manualtrace.WithMinLength(1),
		manualtrace.WithMaxLength(1),
		manualtrace.WithMinOccurrences(2),
		manualtrace.WithEventTypes(event.TypeCommandExecuted),
	)
	result, err := mt.Analyze(events)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.EventsScanned != 2 {
		t.Errorf("EventsScanned = %d, want 2 (commands only)", result.EventsScanned)
	}
}

func TestAnalyze_BelowMinOccurrences_NoPattern(t *testing.T) {
	t0 := time.Date(2026, 9, 30, 14, 0, 0, 0, time.UTC)
	events := []*event.Event{
		mustEvent(t, "o1", event.TypeCommandExecuted, "once-a", t0),
		mustEvent(t, "o2", event.TypeCommandExecuted, "once-b", t0.Add(time.Minute)),
	}

	mt := manualtrace.New(manualtrace.WithMinOccurrences(2))
	result, err := mt.Analyze(events)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.HasPatterns() {
		t.Errorf("single occurrence must not yield pattern: %+v", result.Patterns)
	}
}

func TestAnalyze_Determinism(t *testing.T) {
	t0 := time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC)
	events := []*event.Event{
		mustEvent(t, "d1", event.TypeCommandExecuted, "a", t0),
		mustEvent(t, "d2", event.TypeCommandExecuted, "b", t0.Add(time.Minute)),
		mustEvent(t, "d3", event.TypeCommandExecuted, "a", t0.Add(2*time.Minute)),
		mustEvent(t, "d4", event.TypeCommandExecuted, "b", t0.Add(3*time.Minute)),
		mustEvent(t, "d5", event.TypeCommandExecuted, "c", t0.Add(4*time.Minute)),
		mustEvent(t, "d6", event.TypeCommandExecuted, "a", t0.Add(5*time.Minute)),
		mustEvent(t, "d7", event.TypeCommandExecuted, "b", t0.Add(6*time.Minute)),
	}

	mt := manualtrace.New()
	r1, _ := mt.Analyze(events)
	r2, _ := mt.Analyze(events)

	if len(r1.Patterns) != len(r2.Patterns) {
		t.Fatalf("pattern count %d vs %d", len(r1.Patterns), len(r2.Patterns))
	}
	for i := range r1.Patterns {
		if r1.Patterns[i].SequenceKey() != r2.Patterns[i].SequenceKey() {
			t.Errorf("pattern[%d] key mismatch", i)
		}
		if r1.Patterns[i].Occurrences != r2.Patterns[i].Occurrences {
			t.Errorf("pattern[%d] occurrences mismatch", i)
		}
	}
}

func TestAnalyzeStore_ReadsFromEventStore(t *testing.T) {
	ctx := context.Background()
	store := event.NewMemStore()
	t0 := time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC)

	evts := []*event.Event{
		mustEvent(t, "w0", event.TypeCommandExecuted, "lint", t0),
		mustEvent(t, "w1", event.TypeCommandExecuted, "test", t0.Add(time.Minute)),
		mustEvent(t, "w2", event.TypeCommandExecuted, "lint", t0.Add(2*time.Minute)),
		mustEvent(t, "w3", event.TypeCommandExecuted, "test", t0.Add(3*time.Minute)),
	}
	for _, e := range evts {
		if err := store.Record(ctx, e); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	mt := manualtrace.New(manualtrace.WithMinLength(2), manualtrace.WithMinOccurrences(2))
	result, err := mt.AnalyzeStore(ctx, store, event.Query{})
	if err != nil {
		t.Fatalf("AnalyzeStore: %v", err)
	}
	if !result.HasPatterns() {
		t.Fatal("expected pattern from store events")
	}
}

func TestAnalyzeStore_NilStore(t *testing.T) {
	mt := manualtrace.New()
	_, err := mt.AnalyzeStore(context.Background(), nil, event.Query{})
	if err == nil {
		t.Error("expected error for nil store")
	}
}

func TestLooseCoupling_NoAnalysisImports(t *testing.T) {
	// Compile-time: this package only needs event store. Runtime smoke:
	mt := manualtrace.New()
	result, err := mt.Analyze([]*event.Event{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Version == "" {
		t.Error("Version should be set")
	}
}
