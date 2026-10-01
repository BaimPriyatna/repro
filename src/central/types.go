// Package central provides multi-project central storage, anchor chain resolution,
// and project lifecycle tracking.
package central

import (
	"time"
)

// ProjectID uniquely identifies a project in the central store.
type ProjectID string

// LifecycleStatus describes the activity state of a project.
type LifecycleStatus string

const (
	StatusActive    LifecycleStatus = "active"
	StatusIdle      LifecycleStatus = "idle"
	StatusDormant   LifecycleStatus = "dormant"
	StatusAbandoned LifecycleStatus = "abandoned"
	StatusArchived  LifecycleStatus = "archived"
)

// LifecycleThresholds defines day thresholds for lifecycle transitions.
type LifecycleThresholds struct {
	ActiveDays  int `toml:"active_days" json:"active_days"`
	IdleDays    int `toml:"idle_days" json:"idle_days"`
	DormantDays int `toml:"dormant_days" json:"dormant_days"`
}

// ProjectMeta holds persistent project metadata.
type ProjectMeta struct {
	ID               ProjectID           `toml:"id" json:"id"`
	Name             string              `toml:"name" json:"name"`
	GitInitialCommit string              `toml:"git_initial_commit" json:"git_initial_commit,omitempty"`
	GitRemoteURL     string              `toml:"git_remote_url" json:"git_remote_url,omitempty"`
	KnownPaths       []string            `toml:"known_paths" json:"known_paths"`
	CreatedAt        time.Time           `toml:"created_at" json:"created_at"`
	UpdatedAt        time.Time           `toml:"updated_at" json:"updated_at"`
	LastHostname     string              `toml:"last_hostname" json:"last_hostname"`
	Archived         bool                `toml:"archived" json:"archived"`
	Thresholds       LifecycleThresholds `toml:"thresholds" json:"thresholds"`
}

// ProjectAnchor stores resolution anchors for a project in the central index.
type ProjectAnchor struct {
	ID               ProjectID `toml:"id" json:"id"`
	Name             string    `toml:"name" json:"name"`
	GitInitialCommit string    `toml:"git_initial_commit" json:"git_initial_commit,omitempty"`
	GitRemoteURL     string    `toml:"git_remote_url" json:"git_remote_url,omitempty"`
	KnownPaths       []string  `toml:"known_paths" json:"known_paths"`
}

// AnchorsIndex is the top-level structure stored in anchors.toml.
type AnchorsIndex struct {
	Projects map[string]*ProjectAnchor `toml:"projects" json:"projects"`
}

// ProjectStats provides summary metrics and computed lifecycle status for a project.
type ProjectStats struct {
	Meta            *ProjectMeta    `json:"meta"`
	Status          LifecycleStatus `json:"status"`
	SnapshotCount   int             `json:"snapshot_count"`
	EventCount      int             `json:"event_count"`
	LastActivity    time.Time       `json:"last_activity"`
	DaysSinceActive int             `json:"days_since_active"`
}

// CemeteryEntry represents a project entry in the cemetery listing.
type CemeteryEntry struct {
	ID              ProjectID       `json:"id"`
	Name            string          `json:"name"`
	Status          LifecycleStatus `json:"status"`
	LastPath        string          `json:"last_path"`
	LastActivity    time.Time       `json:"last_activity"`
	DaysSinceActive int             `json:"days_since_active"`
	SnapshotCount   int             `json:"snapshot_count"`
	Archived        bool            `json:"archived"`
}
