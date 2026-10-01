package hasher_test

import (
	"testing"

	"github.com/BaimPriyatna/repro/src/engine/hasher"
)

func TestComputeContentHash_Deterministic(t *testing.T) {
	d1 := map[string]any{
		"b": 2,
		"a": 1,
		"nested": map[string]any{
			"y": "val2",
			"x": "val1",
		},
	}

	d2 := map[string]any{
		"a": 1,
		"nested": map[string]any{
			"x": "val1",
			"y": "val2",
		},
		"b": 2,
	}

	h1, err := hasher.ComputeContentHash(d1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	h2, err := hasher.ComputeContentHash(d2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h1 != h2 {
		t.Errorf("expected deterministic hashes, got h1=%q h2=%q", h1, h2)
	}

	// VerifyContentHash
	ok, err := hasher.VerifyContentHash(d1, h1)
	if err != nil || !ok {
		t.Errorf("expected VerifyContentHash to succeed, got ok=%v, err=%v", ok, err)
	}

	ok, err = hasher.VerifyContentHash(d1, "wrong_hash")
	if err != nil || ok {
		t.Errorf("expected VerifyContentHash to fail for mismatch, got ok=%v, err=%v", ok, err)
	}
}

func TestComputeContentHash_NilAndEmptyMap(t *testing.T) {
	hNil, err := hasher.ComputeContentHash(nil)
	if err != nil {
		t.Fatalf("unexpected error for nil: %v", err)
	}

	hEmpty, err := hasher.ComputeContentHash(map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error for empty: %v", err)
	}

	if hNil != hEmpty {
		t.Errorf("nil and empty map must produce identical content hash: hNil=%q, hEmpty=%q", hNil, hEmpty)
	}
}
