// Package event — query model and ordering for events.
package event

import (
	"sort"
	"strings"
	"time"

	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// SortOrder defines chronological direction for event queries.
type SortOrder string

const (
	// SortAsc orders events from oldest to newest (chronological).
	SortAsc SortOrder = "asc"

	// SortDesc orders events from newest to oldest (reverse chronological, default).
	SortDesc SortOrder = "desc"
)

// Query defines filter criteria and ordering for querying events.
type Query struct {
	// Before matches events with timestamp <= Before (if not zero).
	Before time.Time `json:"before,omitempty"`

	// After matches events with timestamp >= After (if not zero).
	After time.Time `json:"after,omitempty"`

	// Type matches events of a specific Type (if non-empty).
	Type Type `json:"type,omitempty"`

	// Types matches events whose Type is in this list (if non-empty).
	Types []Type `json:"types,omitempty"`

	// Subject matches events with an exact Subject string (if non-empty).
	Subject string `json:"subject,omitempty"`

	// SubjectPrefix matches events whose Subject starts with this prefix (if non-empty).
	SubjectPrefix string `json:"subject_prefix,omitempty"`

	// Source matches events from a specific Source (if non-empty).
	Source Source `json:"source,omitempty"`

	// RelatedSnapshotID matches events associated with a specific snapshot ID (if non-empty).
	RelatedSnapshotID snapshot.ID `json:"related_snapshot_id,omitempty"`

	// HasSnapshot filters events by whether they are associated with any snapshot.
	// If nil, this filter is ignored. If true, only events with HasSnapshot == true match.
	// If false, only events with HasSnapshot == false match.
	HasSnapshot *bool `json:"has_snapshot,omitempty"`

	// Limit caps the maximum number of returned events (0 means unlimited).
	Limit int `json:"limit,omitempty"`

	// Offset skips the first N matching events (0 means no offset).
	Offset int `json:"offset,omitempty"`

	// Order defines timestamp sorting: SortDesc (default, newest first) or SortAsc (oldest first).
	Order SortOrder `json:"order,omitempty"`
}

// Matches evaluates whether an Event satisfies all criteria of the query.
func (q Query) Matches(e *Event) bool {
	if e == nil {
		return false
	}

	// Time window: After (inclusive)
	if !q.After.IsZero() && e.Timestamp.Before(q.After) {
		return false
	}

	// Time window: Before (inclusive)
	if !q.Before.IsZero() && e.Timestamp.After(q.Before) {
		return false
	}

	// Single Type filter
	if q.Type != "" && e.Type != q.Type {
		return false
	}

	// Multi Type filter
	if len(q.Types) > 0 {
		matched := false
		for _, t := range q.Types {
			if e.Type == t {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Source filter
	if q.Source != "" && e.Source != q.Source {
		return false
	}

	// Exact Subject filter
	if q.Subject != "" && e.Subject != q.Subject {
		return false
	}

	// Subject prefix filter
	if q.SubjectPrefix != "" && !strings.HasPrefix(e.Subject, q.SubjectPrefix) {
		return false
	}

	if q.RelatedSnapshotID != "" && e.RelatedSnapshotID != q.RelatedSnapshotID {
		return false
	}

	// Any snapshot presence filter
	if q.HasSnapshot != nil {
		if *q.HasSnapshot != e.HasSnapshot() {
			return false
		}
	}

	return true
}

// SortByTimestamp sorts events in-place according to the given order.
// When timestamps are identical, events are tie-broken deterministically by ID.
func SortByTimestamp(events []*Event, order SortOrder) {
	if len(events) <= 1 {
		return
	}

	if order == SortAsc {
		sort.Slice(events, func(i, j int) bool {
			if events[i].Timestamp.Equal(events[j].Timestamp) {
				return events[i].ID < events[j].ID
			}
			return events[i].Timestamp.Before(events[j].Timestamp)
		})
		return
	}

	// Default to SortDesc (newest first)
	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp.Equal(events[j].Timestamp) {
			return events[i].ID > events[j].ID
		}
		return events[i].Timestamp.After(events[j].Timestamp)
	})
}
