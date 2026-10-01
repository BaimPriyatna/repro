// Package configmerge merges two configuration states and reports conflicts explicitly.
// Conflicts are never silently discarded; merges are deterministic.
package configmerge

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/BaimPriyatna/repro/src/core/errors"
)

const (
	// ConflictReasonValueMismatch means both sides define the same path with different values.
	ConflictReasonValueMismatch = "value_mismatch"

	// ConflictReasonTypeMismatch means both sides define the same path with incompatible types.
	ConflictReasonTypeMismatch = "type_mismatch"
)

// Config is a nested configuration state represented as a string-keyed map.
// Values may be primitives, nested maps, or slices. Unknown value shapes are
// preserved without modification.
type Config map[string]any

// Conflict records an explicit disagreement between Config A and Config B at a
// retained here for the caller to resolve.
type Conflict struct {
	// Path is the dot-separated location of the conflict (e.g. "server.port").
	Path string `json:"path"`

	// ValueA is the value from Config A at Path.
	ValueA any `json:"value_a"`

	// ValueB is the value from Config B at Path.
	ValueB any `json:"value_b"`

	// Reason classifies why the values conflict.
	Reason string `json:"reason"`
}

// Result holds the outcome of a ConfigMerge operation.
type Result struct {
	// Merged is the deterministic union of non-conflicting keys from A and B.
	// Conflicting leaf paths are omitted from Merged and listed in Conflicts.
	Merged Config `json:"merged"`

	// Conflicts lists every path where A and B disagree. Empty when clean.
	Conflicts []Conflict `json:"conflicts"`
}

// HasConflicts reports whether any conflicts were detected.
func (r *Result) HasConflicts() bool {
	return r != nil && len(r.Conflicts) > 0
}

// ConflictCount returns the number of reported conflicts.
func (r *Result) ConflictCount() int {
	if r == nil {
		return 0
	}
	return len(r.Conflicts)
}

// Merge semantically merges config A and config B.
//
// Rules
// Keys present only in A or only in B are taken as-is.
// Keys present in both with deeply equal values are taken once.
// Nested maps are merged recursively.
// Conflicting leaf values (or type mismatches) are omitted from Merged
// and recorded in Conflicts — never silently overwritten.
//
// Nil configs are treated as empty maps. The merge is deterministic: map keys
// are processed in sorted order.
func Merge(a, b Config) (*Result, error) {
	if a == nil {
		a = Config{}
	}
	if b == nil {
		b = Config{}
	}

	merged := Config{}
	conflicts := make([]Conflict, 0)

	mergeMaps("", a, b, merged, &conflicts)

	// Sort conflicts by path for deterministic output.
	sort.SliceStable(conflicts, func(i, j int) bool {
		return conflicts[i].Path < conflicts[j].Path
	})

	return &Result{
		Merged:    merged,
		Conflicts: conflicts,
	}, nil
}

// MergeStrict is like Merge but returns an error when any conflicts exist.
// Useful for callers that require a clean merge before proceeding.
func MergeStrict(a, b Config) (*Result, error) {
	result, err := Merge(a, b)
	if err != nil {
		return nil, err
	}
	if result.HasConflicts() {
		return result, errors.New(
			errors.CodeInvalidInput,
			fmt.Sprintf("configmerge: %d conflict(s) detected; resolve explicitly before applying", result.ConflictCount()),
		)
	}
	return result, nil
}

// mergeMaps recursively merges srcA and srcB into dst, appending conflicts.
func mergeMaps(prefix string, srcA, srcB, dst Config, conflicts *[]Conflict) {
	keys := unionKeys(srcA, srcB)
	sort.Strings(keys)

	for _, key := range keys {
		path := joinPath(prefix, key)
		valA, okA := srcA[key]
		valB, okB := srcB[key]

		switch {
		case okA && !okB:
			dst[key] = cloneValue(valA)
		case !okA && okB:
			dst[key] = cloneValue(valB)
		case okA && okB:
			mergeValues(path, key, valA, valB, dst, conflicts)
		}
	}
}

func mergeValues(path, key string, valA, valB any, dst Config, conflicts *[]Conflict) {
	mapA, isMapA := asStringMap(valA)
	mapB, isMapB := asStringMap(valB)

	// Both sides are nested maps → recurse.
	if isMapA && isMapB {
		child := Config{}
		mergeMaps(path, mapA, mapB, child, conflicts)
		// Always keep the child map (may be empty if all children conflicted).
		// Only omit if the child is empty AND there were conflicts under this path?
		// partial merges of non-conflicting children are useful — keep child
		// even if empty so structure is visible; callers inspect Conflicts.
		dst[key] = child
		return
	}

	// One side map, other not → type mismatch conflict.
	if isMapA != isMapB {
		*conflicts = append(*conflicts, Conflict{
			Path:   path,
			ValueA: cloneValue(valA),
			ValueB: cloneValue(valB),
			Reason: ConflictReasonTypeMismatch,
		})
		return
	}

	// Equal values → take once.
	if deepEqual(valA, valB) {
		dst[key] = cloneValue(valA)
		return
	}

	// Differing leaf values → explicit conflict; omit from merged.
	reason := ConflictReasonValueMismatch
	if reflect.TypeOf(valA) != reflect.TypeOf(valB) && valA != nil && valB != nil {
		reason = ConflictReasonTypeMismatch
	}
	*conflicts = append(*conflicts, Conflict{
		Path:   path,
		ValueA: cloneValue(valA),
		ValueB: cloneValue(valB),
		Reason: reason,
	})
}

func unionKeys(a, b Config) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	return keys
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// asStringMap converts map[string]any or Config into Config.
// Other map kinds (e.g. map[any]any) are not treated as nestable config maps.
func asStringMap(v any) (Config, bool) {
	if v == nil {
		return nil, false
	}
	switch m := v.(type) {
	case Config:
		return m, true
	case map[string]any:
		return Config(m), true
	default:
		return nil, false
	}
}

func deepEqual(a, b any) bool {
	return reflect.DeepEqual(normalize(a), normalize(b))
}

// normalize converts Config alias and any element shapes for stable equality.
func normalize(v any) any {
	switch x := v.(type) {
	case Config:
		return map[string]any(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = normalize(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalize(val)
		}
		return out
	default:
		return v
	}
}

// cloneValue performs a shallow-recursive defensive copy so callers cannot
// mutate Result.Merged / Conflict values through the original config maps.
func cloneValue(v any) any {
	switch x := v.(type) {
	case Config:
		out := make(Config, len(x))
		for k, val := range x {
			out[k] = cloneValue(val)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = cloneValue(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = cloneValue(val)
		}
		return out
	default:
		return v
	}
}

// Paths returns a sorted list of conflict paths.
func (r *Result) Paths() []string {
	if r == nil {
		return nil
	}
	paths := make([]string, len(r.Conflicts))
	for i, c := range r.Conflicts {
		paths[i] = c.Path
	}
	return paths
}

// String returns a compact summary of the merge result.
func (r *Result) String() string {
	if r == nil {
		return "configmerge.Result(nil)"
	}
	if !r.HasConflicts() {
		return fmt.Sprintf("configmerge.Result{merged_keys=%d, conflicts=0}", len(r.Merged))
	}
	return fmt.Sprintf("configmerge.Result{merged_keys=%d, conflicts=%d [%s]}",
		len(r.Merged), len(r.Conflicts), strings.Join(r.Paths(), ", "))
}
