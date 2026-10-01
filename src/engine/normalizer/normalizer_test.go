package normalizer_test

import (
	"bytes"
	"testing"

	"github.com/BaimPriyatna/repro/src/engine/normalizer"
)

func TestNormalize_MapKeyOrdering(t *testing.T) {
	// Construct two maps with different key insertion orders
	m1 := map[string]any{
		"zebra": 1,
		"apple": 2,
		"mango": map[string]any{
			"beta":  "b",
			"alpha": "a",
		},
	}

	m2 := map[string]any{
		"apple": 2,
		"mango": map[string]any{
			"alpha": "a",
			"beta":  "b",
		},
		"zebra": 1,
	}

	b1, err := normalizer.CanonicalJSON(m1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b2, err := normalizer.CanonicalJSON(m2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(b1, b2) {
		t.Errorf("expected canonical JSON to be byte-identical:\nGot b1: %s\nGot b2: %s", string(b1), string(b2))
	}
}

func TestNormalize_SliceHandling(t *testing.T) {
	input := []any{
		map[string]any{"b": 2, "a": 1},
		"scalar",
		123,
	}

	norm := normalizer.Normalize(input)
	slice, ok := norm.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", norm)
	}

	if len(slice) != 3 {
		t.Fatalf("expected slice len 3, got %d", len(slice))
	}
}

func TestNormalize_NilAndEmpty(t *testing.T) {
	if normalizer.Normalize(nil) != nil {
		t.Errorf("expected nil for nil input")
	}

	empty := normalizer.NormalizeMap(nil)
	if empty == nil || len(empty) != 0 {
		t.Errorf("expected non-nil empty map")
	}
}
