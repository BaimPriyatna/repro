package configmerge_test

import (
	"strings"
	"testing"

	"github.com/BaimPriyatna/repro/src/operations/configmerge"
)

func TestMerge_NilConfigsTreatedAsEmpty(t *testing.T) {
	result, err := configmerge.Merge(nil, nil)
	if err != nil {
		t.Fatalf("Merge(nil, nil): %v", err)
	}
	if result.HasConflicts() {
		t.Error("expected no conflicts for empty configs")
	}
	if len(result.Merged) != 0 {
		t.Errorf("expected empty merged, got %v", result.Merged)
	}
}

func TestMerge_DisjointKeysUnion(t *testing.T) {
	a := configmerge.Config{"host": "localhost", "debug": true}
	b := configmerge.Config{"port": 8080, "name": "repro"}

	result, err := configmerge.Merge(a, b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if result.HasConflicts() {
		t.Fatalf("unexpected conflicts: %+v", result.Conflicts)
	}
	if result.Merged["host"] != "localhost" {
		t.Errorf("host = %v", result.Merged["host"])
	}
	if result.Merged["port"] != 8080 {
		t.Errorf("port = %v", result.Merged["port"])
	}
	if result.Merged["debug"] != true {
		t.Errorf("debug = %v", result.Merged["debug"])
	}
	if result.Merged["name"] != "repro" {
		t.Errorf("name = %v", result.Merged["name"])
	}
}

func TestMerge_IdenticalValuesNoConflict(t *testing.T) {
	a := configmerge.Config{
		"server": map[string]any{"port": 8080, "tls": true},
	}
	b := configmerge.Config{
		"server": map[string]any{"port": 8080, "tls": true},
	}

	result, err := configmerge.Merge(a, b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if result.HasConflicts() {
		t.Fatalf("identical values must not conflict: %+v", result.Conflicts)
	}
	server, ok := result.Merged["server"].(map[string]any)
	if !ok {
		// Config alias also acceptable
		if cfg, ok2 := result.Merged["server"].(configmerge.Config); ok2 {
			server = map[string]any(cfg)
		} else {
			t.Fatalf("server type = %T", result.Merged["server"])
		}
	}
	if server["port"] != 8080 {
		t.Errorf("port = %v", server["port"])
	}
}

func TestMerge_ValueMismatchConflict(t *testing.T) {
	a := configmerge.Config{"port": 8080}
	b := configmerge.Config{"port": 9090}

	result, err := configmerge.Merge(a, b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if !result.HasConflicts() {
		t.Fatal("expected conflict for differing port values")
	}
	if result.ConflictCount() != 1 {
		t.Fatalf("ConflictCount = %d, want 1", result.ConflictCount())
	}
	c := result.Conflicts[0]
	if c.Path != "port" {
		t.Errorf("Path = %q, want port", c.Path)
	}
	if c.Reason != configmerge.ConflictReasonValueMismatch {
		t.Errorf("Reason = %q, want %q", c.Reason, configmerge.ConflictReasonValueMismatch)
	}
	if c.ValueA != 8080 || c.ValueB != 9090 {
		t.Errorf("values A=%v B=%v", c.ValueA, c.ValueB)
	}
	// Conflicting key must NOT silently appear in merged with either value.
	if _, ok := result.Merged["port"]; ok {
		t.Error("conflicting key must be omitted from Merged (no silent overwrite)")
	}
}

func TestMerge_TypeMismatchConflict(t *testing.T) {
	a := configmerge.Config{"port": 8080}
	b := configmerge.Config{"port": "8080"}

	result, err := configmerge.Merge(a, b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if result.ConflictCount() != 1 {
		t.Fatalf("expected 1 conflict, got %d", result.ConflictCount())
	}
	if result.Conflicts[0].Reason != configmerge.ConflictReasonTypeMismatch {
		t.Errorf("Reason = %q, want type_mismatch", result.Conflicts[0].Reason)
	}
}

func TestMerge_NestedPartialMerge(t *testing.T) {
	a := configmerge.Config{
		"server": map[string]any{
			"host": "localhost",
			"port": 8080,
		},
		"logging": map[string]any{
			"level": "info",
		},
	}
	b := configmerge.Config{
		"server": map[string]any{
			"host": "localhost",
			"port": 9090, // conflict
			"tls":  true, // only in B
		},
		"logging": map[string]any{
			"level": "info",
		},
	}

	result, err := configmerge.Merge(a, b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}

	if result.ConflictCount() != 1 {
		t.Fatalf("expected 1 conflict, got %d: %+v", result.ConflictCount(), result.Conflicts)
	}
	if result.Conflicts[0].Path != "server.port" {
		t.Errorf("conflict path = %q, want server.port", result.Conflicts[0].Path)
	}

	server := asMap(t, result.Merged["server"])
	if server["host"] != "localhost" {
		t.Errorf("host = %v", server["host"])
	}
	if server["tls"] != true {
		t.Errorf("tls = %v, want true (non-conflicting key from B)", server["tls"])
	}
	if _, ok := server["port"]; ok {
		t.Error("server.port must be omitted from merged")
	}

	logging := asMap(t, result.Merged["logging"])
	if logging["level"] != "info" {
		t.Errorf("logging.level = %v", logging["level"])
	}
}

func TestMerge_MapVsScalarTypeMismatch(t *testing.T) {
	a := configmerge.Config{"db": map[string]any{"host": "localhost"}}
	b := configmerge.Config{"db": "postgres://localhost"}

	result, err := configmerge.Merge(a, b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if result.ConflictCount() != 1 {
		t.Fatalf("expected 1 conflict, got %d", result.ConflictCount())
	}
	if result.Conflicts[0].Reason != configmerge.ConflictReasonTypeMismatch {
		t.Errorf("Reason = %q", result.Conflicts[0].Reason)
	}
	if _, ok := result.Merged["db"]; ok {
		t.Error("type-mismatched key must be omitted from Merged")
	}
}

func TestMerge_SliceValueMismatch(t *testing.T) {
	a := configmerge.Config{"tags": []any{"a", "b"}}
	b := configmerge.Config{"tags": []any{"a", "c"}}

	result, err := configmerge.Merge(a, b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if result.ConflictCount() != 1 {
		t.Fatalf("expected conflict for differing slices, got %d", result.ConflictCount())
	}
	if result.Conflicts[0].Path != "tags" {
		t.Errorf("Path = %q", result.Conflicts[0].Path)
	}
}

func TestMerge_Determinism(t *testing.T) {
	a := configmerge.Config{
		"z": 1,
		"a": map[string]any{"y": 2, "x": 3},
		"m": "only-a",
	}
	b := configmerge.Config{
		"z": 9,                              // conflict
		"a": map[string]any{"y": 2, "x": 4}, // conflict on a.x
		"n": "only-b",
	}

	run := func() *configmerge.Result {
		r, err := configmerge.Merge(a, b)
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		return r
	}

	r1 := run()
	r2 := run()

	if r1.ConflictCount() != r2.ConflictCount() {
		t.Fatalf("conflict count %d vs %d", r1.ConflictCount(), r2.ConflictCount())
	}
	for i := range r1.Conflicts {
		if r1.Conflicts[i].Path != r2.Conflicts[i].Path {
			t.Errorf("conflict[%d].Path = %q vs %q", i, r1.Conflicts[i].Path, r2.Conflicts[i].Path)
		}
		if r1.Conflicts[i].Reason != r2.Conflicts[i].Reason {
			t.Errorf("conflict[%d].Reason = %q vs %q", i, r1.Conflicts[i].Reason, r2.Conflicts[i].Reason)
		}
	}

	// Conflicts must be sorted by path
	paths := r1.Paths()
	for i := 1; i < len(paths); i++ {
		if paths[i-1] > paths[i] {
			t.Errorf("conflicts not sorted: %v", paths)
		}
	}
}

func TestMerge_DoesNotMutateInputs(t *testing.T) {
	a := configmerge.Config{"x": map[string]any{"y": 1}}
	b := configmerge.Config{"x": map[string]any{"y": 1, "z": 2}}

	_, err := configmerge.Merge(a, b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}

	innerA := a["x"].(map[string]any)
	if _, ok := innerA["z"]; ok {
		t.Error("Merge mutated input config A")
	}
}

func TestMergeStrict_ErrorsOnConflicts(t *testing.T) {
	a := configmerge.Config{"port": 1}
	b := configmerge.Config{"port": 2}

	result, err := configmerge.MergeStrict(a, b)
	if err == nil {
		t.Fatal("MergeStrict should error when conflicts exist")
	}
	if result == nil || !result.HasConflicts() {
		t.Fatal("MergeStrict should still return the conflict report")
	}

	cleanA := configmerge.Config{"port": 1}
	cleanB := configmerge.Config{"port": 1, "extra": true}
	result, err = configmerge.MergeStrict(cleanA, cleanB)
	if err != nil {
		t.Fatalf("MergeStrict clean merge: %v", err)
	}
	if result.HasConflicts() {
		t.Error("unexpected conflicts on clean merge")
	}
}

func TestMerge_Standalone_NoAnalysisImports(t *testing.T) {
	// Compile-time / package-level check: this test file only imports configmerge
	// and stdlib. The package itself must not depend on analysis modules.
	// Runtime smoke: merge works with plain maps.
	result, err := configmerge.Merge(
		configmerge.Config{"k": "v"},
		configmerge.Config{"k": "v"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() == "" {
		t.Error("String() should not be empty")
	}
	if !strings.Contains(result.String(), "conflicts=0") {
		t.Errorf("String() = %q", result.String())
	}
}

func TestResult_Paths(t *testing.T) {
	a := configmerge.Config{"b": 1, "a": 1}
	b := configmerge.Config{"b": 2, "a": 2}
	result, _ := configmerge.Merge(a, b)
	paths := result.Paths()
	if len(paths) != 2 {
		t.Fatalf("Paths len = %d", len(paths))
	}
	if paths[0] != "a" || paths[1] != "b" {
		t.Errorf("Paths = %v, want [a b]", paths)
	}
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	switch m := v.(type) {
	case map[string]any:
		return m
	case configmerge.Config:
		return map[string]any(m)
	default:
		t.Fatalf("expected map, got %T", v)
		return nil
	}
}
