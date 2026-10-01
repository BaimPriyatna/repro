package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/hasher"
	"github.com/BaimPriyatna/repro/src/engine/store"
	"github.com/BaimPriyatna/repro/src/presentation/explaindiff"
)

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func setupTestEnvironment(t *testing.T) (string, *store.FileStore, event.Store) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	storeDir := filepath.Join(dataDir, ".repro")
	if err := os.MkdirAll(storeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	snapStore, err := store.NewFileStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}

	evStore, err := event.NewFileStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}

	return dataDir, snapStore, evStore
}

func TestExitCode(t *testing.T) {
	if code := ExitCode(nil); code != 0 {
		t.Fatalf("expected 0 for nil, got %d", code)
	}
	if code := ExitCode(errors.New(errors.CodeInvalidInput, "bad")); code != 2 {
		t.Fatalf("expected 2 for CodeInvalidInput, got %d", code)
	}
	if code := ExitCode(errors.New(errors.CodeNotFound, "missing")); code != 3 {
		t.Fatalf("expected 3 for CodeNotFound, got %d", code)
	}
	if code := ExitCode(errors.New(errors.CodeStorageFailure, "disk")); code != 4 {
		t.Fatalf("expected 4 for CodeStorageFailure, got %d", code)
	}
	if code := ExitCode(errors.New(errors.CodeInternal, "crash")); code != 1 {
		t.Fatalf("expected 1 for CodeInternal, got %d", code)
	}
	if code := ExitCode(fmt.Errorf("generic")); code != 1 {
		t.Fatalf("expected 1 for generic error, got %d", code)
	}
}

func TestCLI_RootHelp(t *testing.T) {
	cmd := RootCommand()
	out, err := executeCommand(cmd, "--help")
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}
	if !strings.Contains(out, "repro") {
		t.Fatalf("expected root help to mention repro, got:\n%s", out)
	}
}

func TestCLI_CaptureAndSnapshot(t *testing.T) {
	dataDir, _, _ := setupTestEnvironment(t)
	cmd := RootCommand()

	// 1. Capture snapshot
	out, err := executeCommand(cmd, "capture", "--local", "--data-dir", dataDir, "--format", "json", "--reason", "test capture")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	var snap snapshot.Snapshot
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatalf("unmarshaling captured snapshot JSON failed: %v\nOutput: %s", err, out)
	}
	if snap.ID == "" {
		t.Fatalf("expected non-empty snapshot ID")
	}

	// 2. Snapshot list
	listOut, err := executeCommand(cmd, "snapshot", "list", "--local", "--data-dir", dataDir)
	if err != nil {
		t.Fatalf("snapshot list failed: %v", err)
	}
	if !strings.Contains(listOut, string(snap.ID)) {
		t.Fatalf("expected snapshot list to contain ID %s, got:\n%s", snap.ID, listOut)
	}

	// 3. Snapshot get
	getOut, err := executeCommand(cmd, "snapshot", "get", string(snap.ID), "--local", "--data-dir", dataDir, "--format", "json")
	if err != nil {
		t.Fatalf("snapshot get failed: %v", err)
	}
	var getSnap snapshot.Snapshot
	if err := json.Unmarshal([]byte(getOut), &getSnap); err != nil {
		t.Fatalf("unmarshaling snapshot get JSON failed: %v", err)
	}
	if getSnap.ID != snap.ID {
		t.Fatalf("expected get ID %s, got %s", snap.ID, getSnap.ID)
	}
}

func TestCLI_DiffHistoryDriftAbsentChangeMap(t *testing.T) {
	dataDir, snapStore, _ := setupTestEnvironment(t)
	cmd := RootCommand()
	ctx := context.Background()

	// Create baseline snapshot
	dataA := map[string]any{"version": "1.0", "active": true}
	hashA, _ := hasher.ComputeContentHash(dataA)
	snapA := &snapshot.Snapshot{
		ID:            snapshot.NewID(),
		Timestamp:     time.Now().UTC().Add(-10 * time.Minute),
		SchemaVersion: 1,
		ContentHash:   hashA,
		Source:        snapshot.SourceManual,
		Data:          dataA,
	}
	if err := snapStore.Store(ctx, snapA); err != nil {
		t.Fatal(err)
	}

	// Create modified snapshot
	dataB := map[string]any{"version": "1.1"}
	hashB, _ := hasher.ComputeContentHash(dataB)
	snapB := &snapshot.Snapshot{
		ID:            snapshot.NewID(),
		Timestamp:     time.Now().UTC(),
		SchemaVersion: 1,
		ParentID:      snapA.ID,
		ContentHash:   hashB,
		Source:        snapshot.SourceManual,
		Data:          dataB,
	}
	if err := snapStore.Store(ctx, snapB); err != nil {
		t.Fatal(err)
	}

	// 1. Diff
	diffOut, err := executeCommand(cmd, "diff", "--local", "--data-dir", dataDir, "--from", string(snapA.ID), "--to", string(snapB.ID), "--format", "json")
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	var diffRes coreanalysis.AnalysisResult
	if err := json.Unmarshal([]byte(diffOut), &diffRes); err != nil {
		t.Fatalf("parsing diff JSON failed: %v", err)
	}
	if len(diffRes.Findings) == 0 {
		t.Fatalf("expected diff findings")
	}

	// 2. History
	histOut, err := executeCommand(cmd, "history", string(snapB.ID), "--local", "--data-dir", dataDir)
	if err != nil {
		t.Fatalf("history failed: %v", err)
	}
	if !strings.Contains(histOut, string(snapA.ID)) || !strings.Contains(histOut, string(snapB.ID)) {
		t.Fatalf("expected history to contain both snapshots, got:\n%s", histOut)
	}

	// 3. Drift
	driftOut, err := executeCommand(cmd, "drift", "--local", "--data-dir", dataDir, "--from", string(snapA.ID), "--format", "json")
	if err != nil {
		t.Fatalf("drift failed: %v", err)
	}
	var driftRes coreanalysis.AnalysisResult
	if err := json.Unmarshal([]byte(driftOut), &driftRes); err != nil {
		t.Fatalf("parsing drift JSON failed: %v", err)
	}

	// 4. Absent
	absentOut, err := executeCommand(cmd, "absent", "--local", "--data-dir", dataDir, "--from", string(snapA.ID), "--to", string(snapB.ID), "--format", "json")
	if err != nil {
		t.Fatalf("absent failed: %v", err)
	}
	var absentRes coreanalysis.AnalysisResult
	if err := json.Unmarshal([]byte(absentOut), &absentRes); err != nil {
		t.Fatalf("parsing absent JSON failed: %v", err)
	}

	// 5. ChangeMap
	changeMapOut, err := executeCommand(cmd, "change-map", string(snapB.ID), "--local", "--data-dir", dataDir, "--format", "json")
	if err != nil {
		t.Fatalf("change-map failed: %v", err)
	}
	var cmRes coreanalysis.AnalysisResult
	if err := json.Unmarshal([]byte(changeMapOut), &cmRes); err != nil {
		t.Fatalf("parsing change-map JSON failed: %v", err)
	}
}

func TestCLI_ConfigMerge(t *testing.T) {
	cmd := RootCommand()
	tmpDir := t.TempDir()

	fileA := filepath.Join(tmpDir, "a.json")
	fileB := filepath.Join(tmpDir, "b.json")

	_ = os.WriteFile(fileA, []byte(`{"app": {"port": 8080, "host": "localhost"}}`), 0o600)
	_ = os.WriteFile(fileB, []byte(`{"app": {"port": 9000, "debug": true}}`), 0o600)

	out, err := executeCommand(cmd, "config-merge", "--a", fileA, "--b", fileB, "--format", "json")
	if err != nil {
		t.Fatalf("config-merge failed: %v", err)
	}
	if !strings.Contains(out, "conflicts") {
		t.Fatalf("expected conflicts in merge output, got:\n%s", out)
	}
}

func TestCLI_GraphAnalysisCommands(t *testing.T) {
	cmd := RootCommand()
	tmpDir := t.TempDir()

	graphFile := filepath.Join(tmpDir, "graph.json")
	graphJSON := `{
		"entities": [
			{"id": "pkg-a", "kind": "package", "name": "pkg-a"},
			{"id": "pkg-b", "kind": "package", "name": "pkg-b"},
			{"id": "cfg-1", "kind": "config", "name": "cfg-1"}
		],
		"relations": [
			{"id": "rel-1", "kind": "depends_on", "from_id": "pkg-a", "to_id": "pkg-b"}
		]
	}`
	if err := os.WriteFile(graphFile, []byte(graphJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	// 1. repair-map
	rmOut, err := executeCommand(cmd, "repair-map", "--graph", graphFile, "--subject", "pkg-a")
	if err != nil {
		t.Fatalf("repair-map failed: %v", err)
	}
	if !strings.Contains(rmOut, "Direct Dependencies") {
		t.Fatalf("expected repair-map to show direct dependencies, got:\n%s", rmOut)
	}

	// 2. impact
	impactOut, err := executeCommand(cmd, "impact", "--graph", graphFile, "--entity", "pkg-b", "--format", "json")
	if err != nil {
		t.Fatalf("impact failed: %v", err)
	}
	var impactRes coreanalysis.AnalysisResult
	if err := json.Unmarshal([]byte(impactOut), &impactRes); err != nil {
		t.Fatalf("parsing impact JSON failed: %v", err)
	}

	// 3. dead-config
	deadOut, err := executeCommand(cmd, "dead-config", "--graph", graphFile, "--format", "json")
	if err != nil {
		t.Fatalf("dead-config failed: %v", err)
	}
	var deadRes coreanalysis.AnalysisResult
	if err := json.Unmarshal([]byte(deadOut), &deadRes); err != nil {
		t.Fatalf("parsing dead-config JSON failed: %v", err)
	}

	// 4. orphan
	orphanOut, err := executeCommand(cmd, "orphan", "--graph", graphFile, "--format", "json")
	if err != nil {
		t.Fatalf("orphan failed: %v", err)
	}
	var orphanRes coreanalysis.AnalysisResult
	if err := json.Unmarshal([]byte(orphanOut), &orphanRes); err != nil {
		t.Fatalf("parsing orphan JSON failed: %v", err)
	}

	// 5. deps
	depsOut, err := executeCommand(cmd, "deps", "--declared", graphFile, "--observed", graphFile, "--format", "json")
	if err != nil {
		t.Fatalf("deps failed: %v", err)
	}
	var depsRes coreanalysis.AnalysisResult
	if err := json.Unmarshal([]byte(depsOut), &depsRes); err != nil {
		t.Fatalf("parsing deps JSON failed: %v", err)
	}
}

func TestCLI_WhyBrokenAndExplanation(t *testing.T) {
	// Reset global flags to prevent pollution from previous tests
	flagGraph = ""
	flagSubject = ""
	flagContext = ""
	flagResult = ""

	dataDir, snapStore, evStore := setupTestEnvironment(t)
	ctx := context.Background()

	dataA := map[string]any{"server.port": 8080}
	hashA, _ := hasher.ComputeContentHash(dataA)
	snapA := &snapshot.Snapshot{
		ID:            snapshot.NewID(),
		Timestamp:     time.Now().UTC().Add(-5 * time.Minute),
		SchemaVersion: 1,
		ContentHash:   hashA,
		Source:        snapshot.SourceManual,
		Data:          dataA,
	}
	_ = snapStore.Store(ctx, snapA)

	dataB := map[string]any{"server.port": 9000}
	hashB, _ := hasher.ComputeContentHash(dataB)
	snapB := &snapshot.Snapshot{
		ID:            snapshot.NewID(),
		Timestamp:     time.Now().UTC(),
		SchemaVersion: 1,
		ParentID:      snapA.ID,
		ContentHash:   hashB,
		Source:        snapshot.SourceManual,
		Data:          dataB,
	}
	_ = snapStore.Store(ctx, snapB)

	ev, _ := event.New(event.TypeConfigChanged, event.SourceUser, "server.port")
	_ = evStore.Record(ctx, ev)

	// Reset command for isolation
	cmd := RootCommand()

	// 1. why-broken
	wbOut, err := executeCommand(cmd, "why-broken", "--local", "--data-dir", dataDir, "--from", string(snapA.ID), "--to", string(snapB.ID), "--format", "json")
	if err != nil {
		t.Fatalf("why-broken failed: %v", err)
	}
	var wbRes coreanalysis.AnalysisResult
	if err := json.Unmarshal([]byte(wbOut), &wbRes); err != nil {
		t.Fatalf("parsing why-broken JSON failed: %v", err)
	}

	// 2. explain-diff
	cmd = RootCommand()
	expOut, err := executeCommand(cmd, "explain-diff", "--local", "--data-dir", dataDir, "--from", string(snapA.ID), "--to", string(snapB.ID), "--format", "json")
	if err != nil {
		t.Fatalf("explain-diff failed: %v", err)
	}
	var exp explaindiff.Explanation
	if err := json.Unmarshal([]byte(expOut), &exp); err != nil {
		t.Fatalf("parsing explain-diff JSON failed: %v", err)
	}

	// 3. human-readable
	cmd = RootCommand()
	hrOut, err := executeCommand(cmd, "human-readable", "--local", "--data-dir", dataDir, "--from", string(snapA.ID), "--to", string(snapB.ID))
	if err != nil {
		t.Fatalf("human-readable failed: %v", err)
	}
	if hrOut == "" {
		t.Fatalf("expected non-empty diagnosis text from human-readable")
	}
}

func TestCLI_ManualTrace(t *testing.T) {
	dataDir, _, evStore := setupTestEnvironment(t)
	cmd := RootCommand()
	ctx := context.Background()

	// Record repeated sequence: command build -> command test
	for i := 0; i < 3; i++ {
		ev1, _ := event.New(event.TypeCommandExecuted, event.SourceUser, "make build", event.WithTimestamp(time.Now().UTC().Add(time.Duration(i*5)*time.Minute)))
		ev2, _ := event.New(event.TypeCommandExecuted, event.SourceUser, "make test", event.WithTimestamp(time.Now().UTC().Add(time.Duration(i*5+1)*time.Minute)))
		_ = evStore.Record(ctx, ev1)
		_ = evStore.Record(ctx, ev2)
	}

	out, err := executeCommand(cmd, "manual-trace", "--local", "--data-dir", dataDir)
	if err != nil {
		t.Fatalf("manual-trace failed: %v", err)
	}
	if !strings.Contains(out, "make build") || !strings.Contains(out, "make test") {
		t.Fatalf("expected manual-trace to detect make build -> make test sequence, got:\n%s", out)
	}
}

func TestCLI_CentralStoreCommands(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "central")
	cmd := RootCommand()

	// 1. init
	initOut, err := executeCommand(cmd, "init", "--data-dir", dataDir, "--name", "cli-test-project")
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if !strings.Contains(initOut, "cli-test-project") {
		t.Fatalf("expected init to output project name, got:\n%s", initOut)
	}

	// 2. list
	listOut, err := executeCommand(cmd, "list", "--data-dir", dataDir)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(listOut, "cli-test-project") {
		t.Fatalf("expected list to show project name, got:\n%s", listOut)
	}

	// 3. status
	statusOut, err := executeCommand(cmd, "status", "--data-dir", dataDir, "--yes")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !strings.Contains(statusOut, "cli-test-project") {
		t.Fatalf("expected status to show project name, got:\n%s", statusOut)
	}

	// 4. cemetery
	cemOut, err := executeCommand(cmd, "cemetery", "--data-dir", dataDir)
	if err != nil {
		t.Fatalf("cemetery failed: %v", err)
	}
	if !strings.Contains(cemOut, "cli-test-project") {
		t.Fatalf("expected cemetery to list project, got:\n%s", cemOut)
	}
}
