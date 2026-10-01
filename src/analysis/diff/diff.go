// Package diff compares two snapshots and produces a deterministic Change list.
//
// Diffing walks normalized maps/slices with dot-path tracking (e.g. "runtime.go.version").
package diff

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// Diff compares two snapshots and returns a detailed change list.
// This is the primary entry point for snapshot comparison.
func Diff(a, b *snapshot.Snapshot) (*analysis.DiffResult, error) {
	if a == nil || b == nil {
		return nil, analysis.ErrInvalidInput
	}

	result := &analysis.DiffResult{
		SnapshotA: a.ID,
		SnapshotB: b.ID,
		Changes:   make([]analysis.Change, 0),
	}

	// Perform recursive diff on snapshot data
	changes := diffValues("", a.Data, b.Data)
	result.Changes = changes

	// Compute summary
	result.Summary = computeSummary(changes)

	return result, nil
}

// diffValues recursively compares two values and produces a change list.
// path is the dot-separated path to the current field being compared.
func diffValues(path string, oldVal, newVal any) []analysis.Change {
	changes := make([]analysis.Change, 0)

	// Handle nil cases
	if oldVal == nil && newVal == nil {
		changes = append(changes, analysis.Change{
			Type:     analysis.ChangeTypeUnchanged,
			Path:     path,
			OldValue: nil,
			NewValue: nil,
		})
		return changes
	}

	if oldVal == nil {
		changes = append(changes, analysis.Change{
			Type:     analysis.ChangeTypeAdded,
			Path:     path,
			OldValue: nil,
			NewValue: newVal,
		})
		return changes
	}

	if newVal == nil {
		changes = append(changes, analysis.Change{
			Type:     analysis.ChangeTypeRemoved,
			Path:     path,
			OldValue: oldVal,
			NewValue: nil,
		})
		return changes
	}

	// Use reflection to handle different types
	oldReflect := reflect.ValueOf(oldVal)
	newReflect := reflect.ValueOf(newVal)

	// Type mismatch - treat as modification
	if oldReflect.Kind() != newReflect.Kind() {
		changes = append(changes, analysis.Change{
			Type:     analysis.ChangeTypeModified,
			Path:     path,
			OldValue: oldVal,
			NewValue: newVal,
		})
		return changes
	}

	switch oldReflect.Kind() {
	case reflect.Map:
		changes = append(changes, diffMaps(path, oldVal, newVal)...)
	case reflect.Slice, reflect.Array:
		changes = append(changes, diffSlices(path, oldVal, newVal)...)
	default:
		// Primitive comparison
		if reflect.DeepEqual(oldVal, newVal) {
			changes = append(changes, analysis.Change{
				Type:     analysis.ChangeTypeUnchanged,
				Path:     path,
				OldValue: oldVal,
				NewValue: newVal,
			})
		} else {
			changes = append(changes, analysis.Change{
				Type:     analysis.ChangeTypeModified,
				Path:     path,
				OldValue: oldVal,
				NewValue: newVal,
			})
		}
	}

	return changes
}

// diffMaps compares two map values recursively.
func diffMaps(path string, oldVal, newVal any) []analysis.Change {
	changes := make([]analysis.Change, 0)

	oldMap, ok1 := oldVal.(map[string]any)
	newMap, ok2 := newVal.(map[string]any)

	if !ok1 || !ok2 {
		// Fallback to simple comparison if type assertion fails
		if reflect.DeepEqual(oldVal, newVal) {
			changes = append(changes, analysis.Change{
				Type:     analysis.ChangeTypeUnchanged,
				Path:     path,
				OldValue: oldVal,
				NewValue: newVal,
			})
		} else {
			changes = append(changes, analysis.Change{
				Type:     analysis.ChangeTypeModified,
				Path:     path,
				OldValue: oldVal,
				NewValue: newVal,
			})
		}
		return changes
	}

	// Collect all keys from both maps
	allKeys := make(map[string]bool)
	for key := range oldMap {
		allKeys[key] = true
	}
	for key := range newMap {
		allKeys[key] = true
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(allKeys))
	for key := range allKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Compare each key
	for _, key := range keys {
		subPath := key
		if path != "" {
			subPath = path + "." + key
		}

		oldValue, oldExists := oldMap[key]
		newValue, newExists := newMap[key]

		if !oldExists && newExists {
			// Key added
			changes = append(changes, analysis.Change{
				Type:     analysis.ChangeTypeAdded,
				Path:     subPath,
				OldValue: nil,
				NewValue: newValue,
			})
		} else if oldExists && !newExists {
			// Key removed
			changes = append(changes, analysis.Change{
				Type:     analysis.ChangeTypeRemoved,
				Path:     subPath,
				OldValue: oldValue,
				NewValue: nil,
			})
		} else {
			// Key exists in both - recurse
			changes = append(changes, diffValues(subPath, oldValue, newValue)...)
		}
	}

	return changes
}

// diffSlices compares two slice/array values.
// For simplicity, this implementation treats slices as changed if lengths differ
// or any element differs. A more sophisticated implementation could use LCS.
func diffSlices(path string, oldVal, newVal any) []analysis.Change {
	changes := make([]analysis.Change, 0)

	oldReflect := reflect.ValueOf(oldVal)
	newReflect := reflect.ValueOf(newVal)

	oldLen := oldReflect.Len()
	newLen := newReflect.Len()

	if oldLen != newLen {
		// Length changed - treat entire slice as modified
		changes = append(changes, analysis.Change{
			Type:     analysis.ChangeTypeModified,
			Path:     path,
			OldValue: oldVal,
			NewValue: newVal,
		})
		return changes
	}

	// Compare element by element
	allUnchanged := true
	for i := 0; i < oldLen; i++ {
		oldElem := oldReflect.Index(i).Interface()
		newElem := newReflect.Index(i).Interface()

		elemPath := fmt.Sprintf("%s[%d]", path, i)
		elemChanges := diffValues(elemPath, oldElem, newElem)

		for _, change := range elemChanges {
			if change.Type != analysis.ChangeTypeUnchanged {
				allUnchanged = false
			}
		}

		changes = append(changes, elemChanges...)
	}

	// If all elements unchanged, collapse to single unchanged entry for the slice
	if allUnchanged {
		changes = []analysis.Change{
			{
				Type:     analysis.ChangeTypeUnchanged,
				Path:     path,
				OldValue: oldVal,
				NewValue: newVal,
			},
		}
	}

	return changes
}

// computeSummary aggregates change counts from a change list.
func computeSummary(changes []analysis.Change) analysis.DiffSummary {
	summary := analysis.DiffSummary{}

	for _, change := range changes {
		switch change.Type {
		case analysis.ChangeTypeAdded:
			summary.Added++
		case analysis.ChangeTypeRemoved:
			summary.Removed++
		case analysis.ChangeTypeModified:
			summary.Modified++
		case analysis.ChangeTypeUnchanged:
			summary.Unchanged++
		}
	}

	summary.TotalItems = len(changes)
	return summary
}

// FilterChanges returns only changes matching the specified types.
// Useful for getting just added/removed/modified changes.
func FilterChanges(changes []analysis.Change, types ...analysis.ChangeType) []analysis.Change {
	typeMap := make(map[analysis.ChangeType]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	filtered := make([]analysis.Change, 0)
	for _, change := range changes {
		if typeMap[change.Type] {
			filtered = append(filtered, change)
		}
	}

	return filtered
}

// GetChangesByPrefix returns all changes whose paths start with the given prefix.
// Useful for isolating changes to a specific subsection (e.g. "runtime.*").
func GetChangesByPrefix(changes []analysis.Change, prefix string) []analysis.Change {
	filtered := make([]analysis.Change, 0)
	for _, change := range changes {
		if strings.HasPrefix(change.Path, prefix) {
			filtered = append(filtered, change)
		}
	}
	return filtered
}
