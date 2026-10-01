package central

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

// CentralStore manages multi-project repository storage and anchor resolution.
//
//nolint:revive // CentralStore name is intentional for clarity in multi-store context
type CentralStore struct {
	mu           sync.RWMutex
	rootDir      string
	projectsDir  string
	anchorsFile  string
	anchorsIndex *AnchorsIndex
}

// NewCentralStore creates or opens a CentralStore rooted at rootDir.
func NewCentralStore(rootDir string) (*CentralStore, error) {
	if rootDir == "" {
		home, _ := os.UserHomeDir()
		rootDir = filepath.Join(home, ".repro", "store")
	}

	projectsDir := filepath.Join(rootDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o750); err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "creating central projects directory", err)
	}

	cs := &CentralStore{
		rootDir:      rootDir,
		projectsDir:  projectsDir,
		anchorsFile:  filepath.Join(rootDir, "anchors.toml"),
		anchorsIndex: &AnchorsIndex{Projects: make(map[string]*ProjectAnchor)},
	}

	if err := cs.loadAnchors(); err != nil {
		return nil, err
	}

	return cs, nil
}

// RootDir returns the root directory of the central store.
func (cs *CentralStore) RootDir() string {
	return cs.rootDir
}

func (cs *CentralStore) loadAnchors() error {
	if _, err := os.Stat(cs.anchorsFile); os.IsNotExist(err) {
		return cs.saveAnchorsUnlocked()
	}

	data, err := os.ReadFile(cs.anchorsFile)
	if err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "reading anchors file", err)
	}

	var idx AnchorsIndex
	if _, err := toml.Decode(string(data), &idx); err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "decoding anchors file", err)
	}

	if idx.Projects == nil {
		idx.Projects = make(map[string]*ProjectAnchor)
	}
	cs.anchorsIndex = &idx
	return nil
}

func (cs *CentralStore) saveAnchorsUnlocked() error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cs.anchorsIndex); err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "encoding anchors index", err)
	}

	tmpFile, err := os.CreateTemp(cs.rootDir, "anchors_*.tmp")
	if err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "creating temporary anchors file", err)
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "writing temporary anchors file", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "closing temporary anchors file", err)
	}

	if err := os.Rename(tmpName, cs.anchorsFile); err != nil {
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "renaming temporary anchors file", err)
	}
	return nil
}

// InitProject registers a new project or connects an existing project directory.
func (cs *CentralStore) InitProject(dir string, name string) (*ProjectMeta, string, error) {
	if dir == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", errors.Wrap(errors.CodeInvalidInput, "resolving absolute path", err)
	}
	absDir = filepath.Clean(absDir)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Check if already resolves to a project
	if meta, err := cs.resolveUnlocked(absDir); err == nil {
		return meta, "", nil
	}

	probe := NewGitProbe(absDir)
	isGit := probe.IsGitRepo()
	var warning string
	if !isGit {
		warning = "Warning: directory is not a Git repository. Anchor resolution will rely on local UUID. Moving or renaming without Git anchors may require manual re-association."
	}

	gitCommit := probe.InitialCommit()
	gitRemote := probe.RemoteURL()

	// Check if matching git commit or remote already exists in anchors
	for _, anchor := range cs.anchorsIndex.Projects {
		if (gitCommit != "" && anchor.GitInitialCommit == gitCommit) ||
			(gitRemote != "" && anchor.GitRemoteURL == gitRemote) {
			meta, err := cs.loadProjectMeta(anchor.ID)
			if err == nil {
				meta.KnownPaths = appendUnique(meta.KnownPaths, absDir)
				meta.UpdatedAt = time.Now().UTC()
				_ = cs.saveProjectMeta(meta)
				anchor.KnownPaths = appendUnique(anchor.KnownPaths, absDir)
				_ = cs.saveAnchorsUnlocked()
				_ = WriteLocalAnchor(absDir, meta.ID)
				return meta, warning, nil
			}
		}
	}

	projID := ProjectID(snapshot.NewID())
	if name == "" {
		name = filepath.Base(absDir)
	}

	now := time.Now().UTC()
	hostname, _ := os.Hostname()

	meta := &ProjectMeta{
		ID:               projID,
		Name:             name,
		GitInitialCommit: gitCommit,
		GitRemoteURL:     gitRemote,
		KnownPaths:       []string{absDir},
		CreatedAt:        now,
		UpdatedAt:        now,
		LastHostname:     hostname,
		Archived:         false,
		Thresholds:       DefaultThresholds(),
	}

	projDir := filepath.Join(cs.projectsDir, string(projID))
	if err := os.MkdirAll(filepath.Join(projDir, "snapshots"), 0o750); err != nil {
		return nil, "", errors.Wrap(errors.CodeStorageFailure, "creating project snapshots dir", err)
	}
	if err := os.MkdirAll(filepath.Join(projDir, "events"), 0o750); err != nil {
		return nil, "", errors.Wrap(errors.CodeStorageFailure, "creating project events dir", err)
	}
	if err := os.MkdirAll(filepath.Join(projDir, "graph"), 0o750); err != nil {
		return nil, "", errors.Wrap(errors.CodeStorageFailure, "creating project graph dir", err)
	}

	if err := cs.saveProjectMeta(meta); err != nil {
		return nil, "", err
	}

	cs.anchorsIndex.Projects[string(projID)] = &ProjectAnchor{
		ID:               projID,
		Name:             name,
		GitInitialCommit: gitCommit,
		GitRemoteURL:     gitRemote,
		KnownPaths:       []string{absDir},
	}
	if err := cs.saveAnchorsUnlocked(); err != nil {
		return nil, "", err
	}

	if err := WriteLocalAnchor(absDir, projID); err != nil {
		return nil, "", errors.Wrap(errors.CodeStorageFailure, "writing local anchor", err)
	}

	return meta, warning, nil
}

// ResolveProject finds the ProjectMeta for a directory using the multi-tier anchor chain.
func (cs *CentralStore) ResolveProject(dir string) (*ProjectMeta, error) {
	if dir == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInvalidInput, "resolving absolute path", err)
	}
	absDir = filepath.Clean(absDir)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	return cs.resolveUnlocked(absDir)
}

func (cs *CentralStore) resolveUnlocked(absDir string) (*ProjectMeta, error) {
	// Tier 1: Local anchor file
	localID := ReadLocalAnchor(absDir)
	if localID != "" {
		if meta, err := cs.loadProjectMeta(ProjectID(localID)); err == nil {
			meta.KnownPaths = appendUnique(meta.KnownPaths, absDir)
			_ = cs.saveProjectMeta(meta)
			if anchor, ok := cs.anchorsIndex.Projects[localID]; ok {
				anchor.KnownPaths = appendUnique(anchor.KnownPaths, absDir)
				_ = cs.saveAnchorsUnlocked()
			}
			return meta, nil
		}
	}

	// Tier 2 & 3: Git anchors
	probe := NewGitProbe(absDir)
	gitCommit := probe.InitialCommit()
	gitRemote := probe.RemoteURL()

	for _, anchor := range cs.anchorsIndex.Projects {
		matched := false
		if gitCommit != "" && anchor.GitInitialCommit != "" && anchor.GitInitialCommit == gitCommit {
			matched = true
		} else if gitRemote != "" && anchor.GitRemoteURL != "" && anchor.GitRemoteURL == gitRemote {
			matched = true
		}

		if matched {
			meta, err := cs.loadProjectMeta(anchor.ID)
			if err == nil {
				meta.KnownPaths = appendUnique(meta.KnownPaths, absDir)
				_ = cs.saveProjectMeta(meta)
				anchor.KnownPaths = appendUnique(anchor.KnownPaths, absDir)
				_ = cs.saveAnchorsUnlocked()
				_ = WriteLocalAnchor(absDir, meta.ID)
				return meta, nil
			}
		}
	}

	// Tier 4: Exact path match in known paths
	for _, anchor := range cs.anchorsIndex.Projects {
		for _, kp := range anchor.KnownPaths {
			if filepath.Clean(kp) == absDir {
				return cs.loadProjectMeta(anchor.ID)
			}
		}
	}

	return nil, errors.New(errors.CodeNotFound, fmt.Sprintf("no project registered for directory: %s", absDir))
}

// AddPath appends a known path to a registered project.
func (cs *CentralStore) AddPath(projectID ProjectID, path string) error {
	absDir, err := filepath.Abs(path)
	if err != nil {
		return errors.Wrap(errors.CodeInvalidInput, "resolving absolute path", err)
	}
	absDir = filepath.Clean(absDir)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	meta, err := cs.loadProjectMeta(projectID)
	if err != nil {
		return err
	}

	meta.KnownPaths = appendUnique(meta.KnownPaths, absDir)
	meta.UpdatedAt = time.Now().UTC()
	if err := cs.saveProjectMeta(meta); err != nil {
		return err
	}

	if anchor, ok := cs.anchorsIndex.Projects[string(projectID)]; ok {
		anchor.KnownPaths = appendUnique(anchor.KnownPaths, absDir)
		_ = cs.saveAnchorsUnlocked()
	}

	_ = WriteLocalAnchor(absDir, projectID)
	return nil
}

// GetProject returns the ProjectMeta for a given ProjectID.
func (cs *CentralStore) GetProject(projectID ProjectID) (*ProjectMeta, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.loadProjectMeta(projectID)
}

func (cs *CentralStore) loadProjectMeta(projectID ProjectID) (*ProjectMeta, error) {
	metaFile := filepath.Join(cs.projectsDir, string(projectID), "meta.toml")
	data, err := os.ReadFile(metaFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.CodeNotFound, fmt.Sprintf("project not found: %s", projectID))
		}
		return nil, errors.Wrap(errors.CodeStorageFailure, "reading project meta", err)
	}

	var meta ProjectMeta
	if _, err := toml.Decode(string(data), &meta); err != nil {
		return nil, errors.Wrap(errors.CodeStorageFailure, "decoding project meta", err)
	}
	return &meta, nil
}

func (cs *CentralStore) saveProjectMeta(meta *ProjectMeta) error {
	projDir := filepath.Join(cs.projectsDir, string(meta.ID))
	if err := os.MkdirAll(projDir, 0o750); err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "creating project directory", err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(meta); err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "encoding project meta", err)
	}

	metaFile := filepath.Join(projDir, "meta.toml")
	tmpFile, err := os.CreateTemp(projDir, "meta_*.tmp")
	if err != nil {
		return errors.Wrap(errors.CodeStorageFailure, "creating temporary meta file", err)
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "writing temporary meta file", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "closing temporary meta file", err)
	}

	if err := os.Rename(tmpName, metaFile); err != nil {
		_ = os.Remove(tmpName)
		return errors.Wrap(errors.CodeStorageFailure, "renaming temporary meta file", err)
	}
	return nil
}

// ListProjects returns all registered projects.
func (cs *CentralStore) ListProjects() ([]*ProjectMeta, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var list []*ProjectMeta
	for id := range cs.anchorsIndex.Projects {
		meta, err := cs.loadProjectMeta(ProjectID(id))
		if err == nil {
			list = append(list, meta)
		}
	}
	return list, nil
}

// ProjectStats returns summary statistics and lifecycle status for a project.
func (cs *CentralStore) ProjectStats(projectID ProjectID) (*ProjectStats, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	meta, err := cs.loadProjectMeta(projectID)
	if err != nil {
		return nil, err
	}

	projDir := filepath.Join(cs.projectsDir, string(projectID))
	snapsDir := filepath.Join(projDir, "snapshots")

	snapCount := 0
	var lastActivity time.Time

	if entries, err := os.ReadDir(snapsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), "tmp_") {
				snapCount++
				if info, err := e.Info(); err == nil {
					if info.ModTime().After(lastActivity) {
						lastActivity = info.ModTime()
					}
				}
			}
		}
	}

	eventCount := 0
	eventsDir := filepath.Join(projDir, "events")
	if entries, err := os.ReadDir(eventsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), "tmp_") {
				eventCount++
				if info, err := e.Info(); err == nil {
					if info.ModTime().After(lastActivity) {
						lastActivity = info.ModTime()
					}
				}
			}
		}
	}

	if lastActivity.IsZero() {
		lastActivity = meta.UpdatedAt
	}
	if lastActivity.IsZero() {
		lastActivity = meta.CreatedAt
	}

	now := time.Now().UTC()
	status := ComputeStatus(now, lastActivity, meta.Thresholds, meta.Archived)

	daysSince := int(now.Sub(lastActivity).Hours() / 24)
	if daysSince < 0 {
		daysSince = 0
	}

	return &ProjectStats{
		Meta:            meta,
		Status:          status,
		SnapshotCount:   snapCount,
		EventCount:      eventCount,
		LastActivity:    lastActivity,
		DaysSinceActive: daysSince,
	}, nil
}

// ListCemetery lists all projects with computed lifecycle status and activity metrics.
func (cs *CentralStore) ListCemetery() ([]*CemeteryEntry, error) {
	projects, err := cs.ListProjects()
	if err != nil {
		return nil, err
	}

	var cemetery []*CemeteryEntry
	for _, p := range projects {
		stats, err := cs.ProjectStats(p.ID)
		if err != nil {
			continue
		}
		lastPath := ""
		if len(p.KnownPaths) > 0 {
			lastPath = p.KnownPaths[len(p.KnownPaths)-1]
		}
		cemetery = append(cemetery, &CemeteryEntry{
			ID:              p.ID,
			Name:            p.Name,
			Status:          stats.Status,
			LastPath:        lastPath,
			LastActivity:    stats.LastActivity,
			DaysSinceActive: stats.DaysSinceActive,
			SnapshotCount:   stats.SnapshotCount,
			Archived:        p.Archived,
		})
	}
	return cemetery, nil
}

// SetThresholds updates the lifecycle thresholds for a project.
func (cs *CentralStore) SetThresholds(projectID ProjectID, thresholds LifecycleThresholds) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	meta, err := cs.loadProjectMeta(projectID)
	if err != nil {
		return err
	}

	meta.Thresholds = thresholds
	meta.UpdatedAt = time.Now().UTC()
	return cs.saveProjectMeta(meta)
}

// Archive marks a project as archived.
func (cs *CentralStore) Archive(projectID ProjectID) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	meta, err := cs.loadProjectMeta(projectID)
	if err != nil {
		return err
	}

	meta.Archived = true
	meta.UpdatedAt = time.Now().UTC()
	return cs.saveProjectMeta(meta)
}

// Unarchive unmarks a project as archived.
func (cs *CentralStore) Unarchive(projectID ProjectID) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	meta, err := cs.loadProjectMeta(projectID)
	if err != nil {
		return err
	}

	meta.Archived = false
	meta.UpdatedAt = time.Now().UTC()
	return cs.saveProjectMeta(meta)
}

// Forget removes a project from the index only, preserving project data on disk.
func (cs *CentralStore) Forget(projectID ProjectID) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, ok := cs.anchorsIndex.Projects[string(projectID)]; !ok {
		return errors.New(errors.CodeNotFound, fmt.Sprintf("project %s not found in anchors index", projectID))
	}

	delete(cs.anchorsIndex.Projects, string(projectID))
	return cs.saveAnchorsUnlocked()
}

// ProjectSnapshotStore returns a FileStore rooted at the project's directory.
func (cs *CentralStore) ProjectSnapshotStore(projectID ProjectID) (*store.FileStore, error) {
	projDir := filepath.Join(cs.projectsDir, string(projectID))
	return store.NewFileStore(projDir)
}

// ProjectEventStore returns a FileStore rooted at the project's directory.
func (cs *CentralStore) ProjectEventStore(projectID ProjectID) (event.Store, error) {
	projDir := filepath.Join(cs.projectsDir, string(projectID))
	return event.NewFileStore(projDir)
}

// CheckHostname checks whether the project was last accessed from the current machine.
func (cs *CentralStore) CheckHostname(meta *ProjectMeta) (bool, string) {
	currentHost, _ := os.Hostname()
	if meta.LastHostname == "" {
		meta.LastHostname = currentHost
		_ = cs.saveProjectMeta(meta)
		return true, currentHost
	}
	return meta.LastHostname == currentHost, currentHost
}

// UpdateHostname updates the project's last hostname to the current host.
func (cs *CentralStore) UpdateHostname(projectID ProjectID) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	meta, err := cs.loadProjectMeta(projectID)
	if err != nil {
		return err
	}
	currentHost, _ := os.Hostname()
	meta.LastHostname = currentHost
	meta.UpdatedAt = time.Now().UTC()
	return cs.saveProjectMeta(meta)
}

// MigrateToCentral copies snapshots and events from localDir/.repro to the central store.
func (cs *CentralStore) MigrateToCentral(localDir string, name string) (*ProjectMeta, error) {
	meta, _, err := cs.InitProject(localDir, name)
	if err != nil {
		return nil, err
	}

	centralSnapStore, err := cs.ProjectSnapshotStore(meta.ID)
	if err != nil {
		return nil, err
	}

	localSnapsDir := filepath.Join(localDir, ".repro", "snapshots")
	if entries, err := os.ReadDir(localSnapsDir); err == nil {
		localStore, err := store.NewFileStore(filepath.Join(localDir, ".repro"))
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
					snapID := snapshot.ID(strings.TrimSuffix(e.Name(), ".json"))
					snap, err := localStore.Load(context.Background(), snapID)
					if err == nil {
						_ = centralSnapStore.Store(context.Background(), snap)
					}
				}
			}
		}
	}

	localEventsDir := filepath.Join(localDir, ".repro", "events")
	if entries, err := os.ReadDir(localEventsDir); err == nil {
		centralEventStore, err := cs.ProjectEventStore(meta.ID)
		if err == nil {
			localEventStore, err := event.NewFileStore(localEventsDir)
			if err == nil {
				events, err := localEventStore.Query(context.Background(), event.Query{})
				if err == nil {
					for _, ev := range events {
						_ = centralEventStore.Record(context.Background(), ev)
					}
				}
			}
		}
		_ = entries
	}

	return meta, nil
}

func appendUnique(slice []string, val string) []string {
	for _, item := range slice {
		if item == val {
			return slice
		}
	}
	return append(slice, val)
}

// CopyFile copies a single file from src to dst.
func CopyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst) //nolint:gosec
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
