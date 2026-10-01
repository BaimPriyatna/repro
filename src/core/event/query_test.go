package event_test

import (
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/core/event"
)

func TestQuery_Matches(t *testing.T) {
	t0 := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 9, 28, 11, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

	sample, _ := event.New(
		event.TypeFileChanged,
		event.SourceWatchdog,
		"src/engine/engine.go",
		event.WithID("evt-sample"),
		event.WithTimestamp(t1),
		event.WithSnapshot("snap-100"),
	)

	// Nil event check
	var nilEvt *event.Event
	if (event.Query{}).Matches(nilEvt) {
		t.Error("expected Matches to return false for nil event")
	}

	// Empty query matches any non-nil event
	if !(event.Query{}).Matches(sample) {
		t.Error("expected empty query to match sample")
	}

	// Time window: After
	if (event.Query{After: t2}).Matches(sample) {
		t.Error("sample occurred at t1, should not match After t2")
	}
	if !(event.Query{After: t0}).Matches(sample) {
		t.Error("sample occurred at t1, should match After t0")
	}
	if !(event.Query{After: t1}).Matches(sample) {
		t.Error("sample occurred at t1, should match After t1 (inclusive)")
	}

	// Time window: Before
	if (event.Query{Before: t0}).Matches(sample) {
		t.Error("sample occurred at t1, should not match Before t0")
	}
	if !(event.Query{Before: t2}).Matches(sample) {
		t.Error("sample occurred at t1, should match Before t2")
	}
	if !(event.Query{Before: t1}).Matches(sample) {
		t.Error("sample occurred at t1, should match Before t1 (inclusive)")
	}

	// Type matching
	if !(event.Query{Type: event.TypeFileChanged}).Matches(sample) {
		t.Error("expected TypeFileChanged to match")
	}
	if (event.Query{Type: event.TypeGitCommit}).Matches(sample) {
		t.Error("TypeGitCommit should not match")
	}

	// Types (multi-type) matching
	if !(event.Query{Types: []event.Type{event.TypeGitCommit, event.TypeFileChanged}}).Matches(sample) {
		t.Error("expected matching Types slice to match")
	}
	if (event.Query{Types: []event.Type{event.TypeGitCommit, event.TypeRuntimeChanged}}).Matches(sample) {
		t.Error("non-matching Types slice should not match")
	}

	// Source matching
	if !(event.Query{Source: event.SourceWatchdog}).Matches(sample) {
		t.Error("expected SourceWatchdog to match")
	}
	if (event.Query{Source: event.SourceUser}).Matches(sample) {
		t.Error("SourceUser should not match")
	}

	// Subject matching
	if !(event.Query{Subject: "src/engine/engine.go"}).Matches(sample) {
		t.Error("expected exact subject match")
	}
	if (event.Query{Subject: "other.go"}).Matches(sample) {
		t.Error("different subject should not match")
	}

	// SubjectPrefix matching
	if !(event.Query{SubjectPrefix: "src/engine"}).Matches(sample) {
		t.Error("expected subject prefix match")
	}
	if (event.Query{SubjectPrefix: "src/cmd"}).Matches(sample) {
		t.Error("different prefix should not match")
	}

	// RelatedSnapshotID matching
	if !(event.Query{RelatedSnapshotID: "snap-100"}).Matches(sample) {
		t.Error("expected snapshot ID match")
	}
	if (event.Query{RelatedSnapshotID: "snap-other"}).Matches(sample) {
		t.Error("different snapshot ID should not match")
	}

	// HasSnapshot boolean matching
	trueBool := true
	falseBool := false
	if !(event.Query{HasSnapshot: &trueBool}).Matches(sample) {
		t.Error("sample has snapshot, expected HasSnapshot: true to match")
	}
	if (event.Query{HasSnapshot: &falseBool}).Matches(sample) {
		t.Error("sample has snapshot, expected HasSnapshot: false to reject")
	}
}

func TestSortByTimestamp_Deterministic(t *testing.T) {
	t1 := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 28, 11, 0, 0, 0, time.UTC)

	// Create events where e2 and e3 share identical timestamps
	e1, _ := event.New(event.TypeFileChanged, event.SourceWatchdog, "sub1", event.WithID("evt-1"), event.WithTimestamp(t1))
	e2, _ := event.New(event.TypeFileChanged, event.SourceWatchdog, "sub2", event.WithID("evt-2"), event.WithTimestamp(t2))
	e3, _ := event.New(event.TypeFileChanged, event.SourceWatchdog, "sub3", event.WithID("evt-3"), event.WithTimestamp(t2))

	// Descending (newest first): t2 events first. Between e2 and e3 (same timestamp), e3 > e2 by ID
	events := []*event.Event{e1, e2, e3}
	event.SortByTimestamp(events, event.SortDesc)
	if events[0].ID != "evt-3" || events[1].ID != "evt-2" || events[2].ID != "evt-1" {
		t.Fatalf("unexpected SortDesc order: [%s, %s, %s]", events[0].ID, events[1].ID, events[2].ID)
	}

	// Ascending (oldest first): t1 first, then tie-broken ascending by ID: evt-2 then evt-3
	event.SortByTimestamp(events, event.SortAsc)
	if events[0].ID != "evt-1" || events[1].ID != "evt-2" || events[2].ID != "evt-3" {
		t.Fatalf("unexpected SortAsc order: [%s, %s, %s]", events[0].ID, events[1].ID, events[2].ID)
	}

	// Single item sort does not panic
	single := []*event.Event{e1}
	event.SortByTimestamp(single, event.SortAsc)
	if len(single) != 1 {
		t.Fatal("unexpected length after single item sort")
	}
}
