// Package collector — Git state collector.
//
// GitCollector runs "git rev-parse HEAD" and "git status --porcelain" to
// capture the current commit hash and working-tree cleanliness.
//
// If Git is unavailable or the working directory is not a repository, the
// collector returns a Result with an explanatory Warning rather than an error,
// since a missing.git directory is a valid (non-error) condition.
package collector

import (
	"context"
	"os/exec"
	"strings"
)

// GitInfo holds the captured Git repository state.
type GitInfo struct {
	// Commit is the full SHA-1 of HEAD, or empty if unavailable.
	Commit string `json:"commit"`

	// Branch is the current branch name, or empty/detached-head description.
	Branch string `json:"branch"`

	// IsDirty is true when the working tree has uncommitted changes.
	IsDirty bool `json:"is_dirty"`

	// Available is false when git is not installed or the directory is not
	// a repository. When false, all other fields are zero values.
	Available bool `json:"available"`
}

// GitCollector captures the current Git repository state.
// It runs git sub-commands and is therefore I/O-bound; always pass a context
// with an appropriate deadline.
type GitCollector struct {
	// WorkDir is the directory to run git in. Empty means the process's cwd.
	WorkDir string
}

// NewGitCollector returns a GitCollector that runs in dir (empty = process cwd).
func NewGitCollector(dir string) *GitCollector {
	return &GitCollector{WorkDir: dir}
}

// Name implements Collector.
func (c *GitCollector) Name() string { return "git" }

// Collect implements Collector.
// A non-nil error is returned only for truly unexpected conditions (e.g. context
// cancellation). A missing git binary or non-repo directory yields a Result
// with Available=false and a non-empty Warning.
func (c *GitCollector) Collect(ctx context.Context) (Result, error) {
	// Quick check: is git available?
	if _, err := exec.LookPath("git"); err != nil {
		return Result{
			Data:    GitInfo{Available: false},
			Warning: "git executable not found; git state not captured",
		}, nil
	}

	commit, err := c.run(ctx, "rev-parse", "HEAD")
	if err != nil {
		// Not a git repo or HEAD doesn't exist yet.
		return Result{
			Data:    GitInfo{Available: false},
			Warning: "not a git repository or no commits yet; git state not captured",
		}, nil
	}

	branch, _ := c.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	statusOut, _ := c.run(ctx, "status", "--porcelain")
	isDirty := strings.TrimSpace(statusOut) != ""

	return Result{
		Data: GitInfo{
			Commit:    strings.TrimSpace(commit),
			Branch:    strings.TrimSpace(branch),
			IsDirty:   isDirty,
			Available: true,
		},
	}, nil
}

// run executes git with args in c.WorkDir and returns combined stdout.
// Returns an error if the command fails (non-zero exit or context cancelled).
func (c *GitCollector) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // args are caller-controlled constants
	if c.WorkDir != "" {
		cmd.Dir = c.WorkDir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
