package central

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitProbe captures Git identity markers for anchor resolution.
type GitProbe struct {
	WorkDir string
}

// NewGitProbe creates a Git probe for the given directory.
func NewGitProbe(dir string) *GitProbe {
	return &GitProbe{WorkDir: dir}
}

// InitialCommit returns the initial commit SHA of the repository, or empty string.
func (p *GitProbe) InitialCommit() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-list", "--max-parents=0", "HEAD")
	if p.WorkDir != "" {
		cmd.Dir = p.WorkDir
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[len(lines)-1])
}

// RemoteURL returns the origin remote URL of the repository, or empty string.
func (p *GitProbe) RemoteURL() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "config", "--get", "remote.origin.url")
	if p.WorkDir != "" {
		cmd.Dir = p.WorkDir
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// IsGitRepo reports whether the given directory is inside a Git repository.
func (p *GitProbe) IsGitRepo() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	if p.WorkDir != "" {
		cmd.Dir = p.WorkDir
	}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// ReadLocalAnchor reads the project UUID stored in .repro/anchor or .repro/id.
func ReadLocalAnchor(dir string) string {
	for _, name := range []string{"anchor", "id"} {
		p := filepath.Join(dir, ".repro", name)
		data, err := os.ReadFile(p)
		if err == nil {
			id := strings.TrimSpace(string(data))
			if id != "" {
				return id
			}
		}
	}
	return ""
}

// WriteLocalAnchor saves the project UUID into .repro/anchor.
func WriteLocalAnchor(dir string, id ProjectID) error {
	reproDir := filepath.Join(dir, ".repro")
	if err := os.MkdirAll(reproDir, 0o750); err != nil {
		return err
	}
	anchorPath := filepath.Join(reproDir, "anchor")
	return os.WriteFile(anchorPath, []byte(strings.TrimSpace(string(id))+"\n"), 0o600)
}
