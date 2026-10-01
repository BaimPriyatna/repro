package analysis_test

import (
	"testing"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

func TestChangeType(t *testing.T) {
	tests := []struct {
		name     string
		ct       analysis.ChangeType
		expected string
	}{
		{"added", analysis.ChangeTypeAdded, "added"},
		{"removed", analysis.ChangeTypeRemoved, "removed"},
		{"modified", analysis.ChangeTypeModified, "modified"},
		{"unchanged", analysis.ChangeTypeUnchanged, "unchanged"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.ct) != tt.expected {
				t.Errorf("ChangeType = %v, want %v", tt.ct, tt.expected)
			}
		})
	}
}

func TestDiffResult_HasChanges(t *testing.T) {
	tests := []struct {
		name     string
		summary  analysis.DiffSummary
		expected bool
	}{
		{
			name: "has added changes",
			summary: analysis.DiffSummary{
				Added: 1,
			},
			expected: true,
		},
		{
			name: "has removed changes",
			summary: analysis.DiffSummary{
				Removed: 1,
			},
			expected: true,
		},
		{
			name: "has modified changes",
			summary: analysis.DiffSummary{
				Modified: 1,
			},
			expected: true,
		},
		{
			name: "only unchanged",
			summary: analysis.DiffSummary{
				Unchanged: 5,
			},
			expected: false,
		},
		{
			name:     "empty",
			summary:  analysis.DiffSummary{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dr := &analysis.DiffResult{
				Summary: tt.summary,
			}
			if got := dr.HasChanges(); got != tt.expected {
				t.Errorf("HasChanges() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHistoryChain_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		chain    *analysis.HistoryChain
		expected bool
	}{
		{
			name:     "empty chain",
			chain:    &analysis.HistoryChain{},
			expected: true,
		},
		{
			name: "non-empty chain",
			chain: &analysis.HistoryChain{
				Snapshots: []*snapshot.Snapshot{
					{ID: "snap-1"},
				},
				Depth: 1,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chain.IsEmpty(); got != tt.expected {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.expected)
			}
		})
	}
}
