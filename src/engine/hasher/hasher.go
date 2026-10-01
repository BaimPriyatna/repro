// Package hasher provides deterministic content hashing for Repro snapshots.
//
// Every snapshot carries a ContentHash calculated deterministically over its Data
// payload. This hash is used to detect silent corruption and to
// verify that snapshots can be reloaded byte-identical with zero drift.
package hasher

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/BaimPriyatna/repro/src/engine/normalizer"
)

// HashBytes computes the SHA-256 hash of a byte slice and returns it as a lowercase hex string.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ComputeContentHash normalizes the data map, serializes it to canonical JSON,
// and computes its SHA-256 hash.
func ComputeContentHash(data map[string]any) (string, error) {
	if data == nil {
		data = map[string]any{}
	}

	canonical, err := normalizer.CanonicalJSON(data)
	if err != nil {
		return "", err
	}

	return HashBytes(canonical), nil
}

// VerifyContentHash checks whether the given data payload matches expectedHash.
func VerifyContentHash(data map[string]any, expectedHash string) (bool, error) {
	computed, err := ComputeContentHash(data)
	if err != nil {
		return false, err
	}
	return computed == expectedHash, nil
}
