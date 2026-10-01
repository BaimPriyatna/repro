package central

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/hasher"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

func TestComputeStatus(t *testing.T) {
	thresholds := DefaultThresholds()
	now := time.Now().UTC()

	// Active: 2 days ago
	activeTime := now.Add(-2 * 24 * time.Hour)
	if s := ComputeStatus(now, activeTime, thresholds, false); s != StatusActive {
		t.Fatalf("expected %s, got %s", StatusActive, s)
	}

	// Idle: 15 days ago
	idleTime := now.Add(-15 * 24 * time.Hour)
	if s := ComputeStatus(now, idleTime, thresholds, false); s != StatusIdle {
		t.Fatalf("expected %s, got %s", StatusIdle, s)
	}

	// Dormant: 45 days ago
	dormantTime := now.Add(-45 * 24 * time.Hour)
	if s := ComputeStatus(now, dormantTime, thresholds, false); s != StatusDormant {
		t.Fatalf("expected %s, got %s", StatusDormant, s)
	}

	// Abandoned: 100 days ago
	abandonedTime := now.Add(-100 * 24 * time.Hour)
	if s := ComputeStatus(now, abandonedTime, thresholds, false); s != StatusAbandoned {
		t.Fatalf("expected %s, got %s", StatusAbandoned, s)
	}

	// Archived: regardless of time
	if s := ComputeStatus(now, activeTime, thresholds, true); s != StatusArchived {
		t.Fatalf("expected %s, got %s", StatusArchived, s)
	}
}

func TestCentralStore_InitAndResolve(t *testing.T) {
	tmpDir := t.TempDir()
	centralDir := filepath.Join(tmpDir, "central")
	projectDir := filepath.Join(tmpDir, "my-project")
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		t.Fatal(err)
	}

	cs, err := NewCentralStore(centralDir)
	if err != nil {
		t.Fatalf("NewCentralStore failed: %v", err)
	}

	meta, warn, err := cs.InitProject(projectDir, "my-project")
	if err != nil {
		t.Fatalf("InitProject failed: %v", err)
	}
	if meta.Name != "my-project" {
		t.Fatalf("expected name my-project, got %s", meta.Name)
	}
	if warn == "" {
		t.Logf("expected warning for non-git folder")
	}

	// Resolve by path
	resolved, err := cs.ResolveProject(projectDir)
	if err != nil {
		t.Fatalf("ResolveProject failed: %v", err)
	}
	if resolved.ID != meta.ID {
		t.Fatalf("expected ID %s, got %s", meta.ID, resolved.ID)
	}

	// Read local anchor
	localAnchor := ReadLocalAnchor(projectDir)
	if localAnchor != string(meta.ID) {
		t.Fatalf("expected local anchor %s, got %s", meta.ID, localAnchor)
	}

	// Rename/move project folder
	movedDir := filepath.Join(tmpDir, "moved-project")
	if err := os.Rename(projectDir, movedDir); err != nil {
		t.Fatal(err)
	}

	// Resolve moved project via local anchor
	resolvedMoved, err := cs.ResolveProject(movedDir)
	if err != nil {
		t.Fatalf("failed to resolve moved project: %v", err)
	}
	if resolvedMoved.ID != meta.ID {
		t.Fatalf("expected ID %s, got %s", meta.ID, resolvedMoved.ID)
	}
	// Check that movedDir was appended to known paths
	found := false
	for _, p := range resolvedMoved.KnownPaths {
		if p == movedDir {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s in known paths: %v", movedDir, resolvedMoved.KnownPaths)
	}
}

func TestCentralStore_NonDestructiveForget(t *testing.T) {
	tmpDir := t.TempDir()
	centralDir := filepath.Join(tmpDir, "central")
	projectDir := filepath.Join(tmpDir, "proj")
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		t.Fatal(err)
	}

	cs, err := NewCentralStore(centralDir)
	if err != nil {
		t.Fatal(err)
	}

	meta, _, err := cs.InitProject(projectDir, "proj")
	if err != nil {
		t.Fatal(err)
	}

	// Store a snapshot in central store
	snapStore, err := cs.ProjectSnapshotStore(meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	testData := map[string]any{"os": "linux"}
	contentHash, err := hasher.ComputeContentHash(testData)
	if err != nil {
		t.Fatal(err)
	}

	snap := &snapshot.Snapshot{
		ID:            snapshot.NewID(),
		Timestamp:     time.Now().UTC(),
		SchemaVersion: 1,
		ContentHash:   contentHash,
		Source:        snapshot.SourceManual,
		Data:          testData,
	}

	if err := snapStore.Store(context.Background(), snap); err != nil {
		t.Fatalf("Store snapshot failed: %v", err)
	}

	// Forget project
	if err := cs.Forget(meta.ID); err != nil {
		t.Fatalf("Forget failed: %v", err)
	}

	// Project should not be found in index
	projects, err := cs.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range projects {
		if p.ID == meta.ID {
			t.Fatalf("project should not be listed after forget")
		}
	}

	// Invariant: Snapshot file must still exist on disk!
	rawSnapStore, err := cs.ProjectSnapshotStore(meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := rawSnapStore.Load(context.Background(), snap.ID)
	if err != nil {
		t.Fatalf("historical snapshot was deleted during forget: %v", err)
	}
	if loaded.ID != snap.ID {
		t.Fatalf("loaded snap ID mismatch: expected %s, got %s", snap.ID, loaded.ID)
	}
}

func TestCentralStore_StatsAndCemetery(t *testing.T) {
	tmpDir := t.TempDir()
	centralDir := filepath.Join(tmpDir, "central")
	projectDir := filepath.Join(tmpDir, "proj")
	_ = os.MkdirAll(projectDir, 0o750)

	cs, err := NewCentralStore(centralDir)
	if err != nil {
		t.Fatal(err)
	}

	meta, _, err := cs.InitProject(projectDir, "cemetery-proj")
	if err != nil {
		t.Fatal(err)
	}

	// Add an event
	evStore, err := cs.ProjectEventStore(meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := event.New(event.TypeCommandExecuted, event.SourceUser, "subject")
	if err != nil {
		t.Fatal(err)
	}
	if err := evStore.Record(context.Background(), ev); err != nil {
		t.Fatal(err)
	}

	stats, err := cs.ProjectStats(meta.ID)
	if err != nil {
		t.Fatalf("ProjectStats failed: %v", err)
	}
	if stats.EventCount != 1 {
		t.Fatalf("expected 1 event, got %d", stats.EventCount)
	}
	if stats.Status != StatusActive {
		t.Fatalf("expected status active, got %s", stats.Status)
	}

	cemetery, err := cs.ListCemetery()
	if err != nil {
		t.Fatalf("ListCemetery failed: %v", err)
	}
	if len(cemetery) != 1 {
		t.Fatalf("expected 1 cemetery entry, got %d", len(cemetery))
	}
	if cemetery[0].Name != "cemetery-proj" {
		t.Fatalf("expected cemetery-proj, got %s", cemetery[0].Name)
	}

	// Test Archive
	if err := cs.Archive(meta.ID); err != nil {
		t.Fatal(err)
	}
	statsAfterArchive, err := cs.ProjectStats(meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if statsAfterArchive.Status != StatusArchived {
		t.Fatalf("expected status archived, got %s", statsAfterArchive.Status)
	}
}

func TestCentralStore_MigrateToCentral(t *testing.T) {
	tmpDir := t.TempDir()
	centralDir := filepath.Join(tmpDir, "central")
	localDir := filepath.Join(tmpDir, "local")
	localSnaps := filepath.Join(localDir, ".repro")
	_ = os.MkdirAll(localSnaps, 0o750)

	// Create local snapshot
	localStore, err := csNewFileStore(localSnaps)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"git": "clean"}
	hash, _ := hasher.ComputeContentHash(data)
	snap := &snapshot.Snapshot{
		ID:            snapshot.NewID(),
		Timestamp:     time.Now().UTC(),
		SchemaVersion: 1,
		ContentHash:   hash,
		Source:        snapshot.SourceManual,
		Data:          data,
	}
	if err := localStore.Store(context.Background(), snap); err != nil {
		t.Fatal(err)
	}

	cs, err := NewCentralStore(centralDir)
	if err != nil {
		t.Fatal(err)
	}

	meta, err := cs.MigrateToCentral(localDir, "migrated")
	if err != nil {
		t.Fatalf("MigrateToCentral failed: %v", err)
	}

	centralSnapStore, err := cs.ProjectSnapshotStore(meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	migratedSnap, err := centralSnapStore.Load(context.Background(), snap.ID)
	if err != nil {
		t.Fatalf("failed to load migrated snapshot: %v", err)
	}
	if migratedSnap.ID != snap.ID {
		t.Fatalf("expected snapshot %s, got %s", snap.ID, migratedSnap.ID)
	}
}

func csNewFileStore(dir string) (*store.FileStore, error) {
	return store.NewFileStore(dir)
}

func TestCentralStore_GitAnchorReconstruction(t *testing.T) {
	tmpDir := t.TempDir()
	centralDir := filepath.Join(tmpDir, "central")
	gitDir := filepath.Join(tmpDir, "git-repo")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Initialize git repo
	runGit := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git command failed (%v): %s", err, string(out))
		}
	}

	runGit(gitDir, "init")
	runGit(gitDir, "config", "user.name", "Test")
	runGit(gitDir, "config", "user.email", "test@example.com")
	testFile := filepath.Join(gitDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(gitDir, "add", "README.md")
	runGit(gitDir, "commit", "-m", "initial commit")

	cs, err := NewCentralStore(centralDir)
	if err != nil {
		t.Fatal(err)
	}

	meta, _, err := cs.InitProject(gitDir, "git-repo")
	if err != nil {
		t.Fatal(err)
	}
	if meta.GitInitialCommit == "" {
		t.Fatalf("expected git initial commit to be captured")
	}

	// Delete local .repro directory completely to simulate anchor loss
	localRepro := filepath.Join(gitDir, ".repro")
	if err := os.RemoveAll(localRepro); err != nil {
		t.Fatal(err)
	}

	// Now resolve project again: should reconstruct via Git initial commit!
	reconstructed, err := cs.ResolveProject(gitDir)
	if err != nil {
		t.Fatalf("failed to resolve project after local anchor loss: %v", err)
	}
	if reconstructed.ID != meta.ID {
		t.Fatalf("expected ID %s, got %s", meta.ID, reconstructed.ID)
	}

	// Check local anchor is recreated
	recreatedAnchor := ReadLocalAnchor(gitDir)
	if recreatedAnchor != string(meta.ID) {
		t.Fatalf("expected local anchor to be recreated with %s, got %s", meta.ID, recreatedAnchor)
	}
}
