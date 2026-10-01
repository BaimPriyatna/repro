// Package normalizer provides deterministic normalization of snapshot data.
//
// Repro requires deterministic analysis. Normalization guarantees that
// semantically identical data structures produce identical serialized representations,
// regardless of key insertion order, nested map variants, or struct vs map representations.
package normalizer

import (
	"encoding/json"
	"sort"
)

// Normalize recursively traverses data and produces a canonical, normalized form:
// Map keys are sorted alphabetically.
// Structs and custom types are converted to generic JSON representations.
// Numeric types are unified to standard JSON numbers (float64).
// Nested structures (slices, maps) are recursively normalized.
// Nil maps and empty maps are represented consistently.
func Normalize(val any) any {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case string, bool, float64:
		return v
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case map[string]any:
		return normalizeStringMap(v)
	case []any:
		normalizedSlice := make([]any, len(v))
		for i, elem := range v {
			normalizedSlice[i] = Normalize(elem)
		}
		return normalizedSlice
	default:
		// For structs, maps with non-string keys, or custom types,
		// round-trip through JSON marshaling to produce consistent generic structures.
		b, err := json.Marshal(v)
		if err != nil {
			return val
		}
		var generic any
		if err := json.Unmarshal(b, &generic); err != nil {
			return val
		}
		return Normalize(generic)
	}
}

// NormalizeMap normalizes a map[string]any recursively.
func NormalizeMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return normalizeStringMap(m)
}

func normalizeStringMap(m map[string]any) map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make(map[string]any, len(m))
	for _, k := range keys {
		out[k] = Normalize(m[k])
	}
	return out
}

// CanonicalJSON converts any value into deterministic, canonical JSON bytes.
// Map keys in standard Go encoding/json are automatically sorted, but passing
// through Normalize guarantees nested structure normalization and consistency.
func CanonicalJSON(v any) ([]byte, error) {
	normalized := Normalize(v)
	return json.Marshal(normalized)
}
