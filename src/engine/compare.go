// Package engine — snapshot comparison and diffing.
package engine

import (
	"bytes"
	"sort"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/normalizer"
)

// KeyDiff represents differences in a specific collector key between two snapshots.
type KeyDiff struct {
	Key      string `json:"key"`
	OldValue any    `json:"old_value,omitempty"`
	NewValue any    `json:"new_value,omitempty"`
}

// Comparison represents the structural and content difference between two snapshots.
type Comparison struct {
	// BaseID is the ID of snapshot A.
	BaseID snapshot.ID `json:"base_id"`

	// TargetID is the ID of snapshot B.
	TargetID snapshot.ID `json:"target_id"`

	// Identical is true if both snapshots have identical data payloads (zero drift).
	Identical bool `json:"identical"`

	// AddedKeys lists collector keys present in B but not in A.
	AddedKeys []string `json:"added_keys"`

	// RemovedKeys lists collector keys present in A but not in B.
	RemovedKeys []string `json:"removed_keys"`

	// ModifiedKeys lists collector keys present in both with different contents.
	ModifiedKeys []string `json:"modified_keys"`

	// UnchangedKeys lists collector keys present in both with identical contents.
	UnchangedKeys []string `json:"unchanged_keys"`

	// DiffDetails contains specific old and new representations for modified keys.
	DiffDetails map[string]KeyDiff `json:"diff_details,omitempty"`
}

// Compare computes differences between snapshot a (base) and snapshot b (target).
// If a and b are identical, Identical is true with zero modified/added/removed keys.
func Compare(a, b *snapshot.Snapshot) (*Comparison, error) {
	if a == nil || b == nil {
		return nil, errors.New(errors.CodeInvalidInput, "cannot compare nil snapshot")
	}

	comp := &Comparison{
		BaseID:        a.ID,
		TargetID:      b.ID,
		AddedKeys:     []string{},
		RemovedKeys:   []string{},
		ModifiedKeys:  []string{},
		UnchangedKeys: []string{},
		DiffDetails:   make(map[string]KeyDiff),
	}

	// If content hashes match, snapshots have zero drift
	if a.ContentHash != "" && a.ContentHash == b.ContentHash {
		comp.Identical = true
		for k := range a.Data {
			comp.UnchangedKeys = append(comp.UnchangedKeys, k)
		}
		sort.Strings(comp.UnchangedKeys)
		return comp, nil
	}

	allKeys := make(map[string]struct{})
	for k := range a.Data {
		allKeys[k] = struct{}{}
	}
	for k := range b.Data {
		allKeys[k] = struct{}{}
	}

	for k := range allKeys {
		valA, inA := a.Data[k]
		valB, inB := b.Data[k]

		if !inA && inB {
			comp.AddedKeys = append(comp.AddedKeys, k)
			comp.DiffDetails[k] = KeyDiff{Key: k, NewValue: valB}
			continue
		}
		if inA && !inB {
			comp.RemovedKeys = append(comp.RemovedKeys, k)
			comp.DiffDetails[k] = KeyDiff{Key: k, OldValue: valA}
			continue
		}

		// Both present — check canonical JSON representation for equality
		jsonA, errA := normalizer.CanonicalJSON(valA)
		jsonB, errB := normalizer.CanonicalJSON(valB)
		if errA != nil || errB != nil || !bytes.Equal(jsonA, jsonB) {
			comp.ModifiedKeys = append(comp.ModifiedKeys, k)
			comp.DiffDetails[k] = KeyDiff{Key: k, OldValue: valA, NewValue: valB}
		} else {
			comp.UnchangedKeys = append(comp.UnchangedKeys, k)
		}
	}

	sort.Strings(comp.AddedKeys)
	sort.Strings(comp.RemovedKeys)
	sort.Strings(comp.ModifiedKeys)
	sort.Strings(comp.UnchangedKeys)

	comp.Identical = len(comp.AddedKeys) == 0 && len(comp.RemovedKeys) == 0 && len(comp.ModifiedKeys) == 0
	return comp, nil
}
