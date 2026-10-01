package collector_test

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/BaimPriyatna/repro/src/engine/collector"
)

func TestOSCollector(t *testing.T) {
	c := collector.NewOSCollector()
	if c.Name() != "os" {
		t.Fatalf("expected name 'os', got %q", c.Name())
	}

	res, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, ok := res.Data.(collector.OSInfo)
	if !ok {
		t.Fatalf("expected OSInfo data type, got %T", res.Data)
	}

	if info.OS != runtime.GOOS {
		t.Errorf("expected OS %q, got %q", runtime.GOOS, info.OS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("expected Arch %q, got %q", runtime.GOARCH, info.Arch)
	}
}

func TestRuntimeCollector(t *testing.T) {
	c := collector.NewRuntimeCollector()
	if c.Name() != "runtime" {
		t.Fatalf("expected name 'runtime', got %q", c.Name())
	}

	res, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, ok := res.Data.(collector.RuntimeInfo)
	if !ok {
		t.Fatalf("expected RuntimeInfo data type, got %T", res.Data)
	}

	if info.GoVersion != runtime.Version() {
		t.Errorf("expected GoVersion %q, got %q", runtime.Version(), info.GoVersion)
	}
}

func TestEnvCollector_DisabledByDefault(t *testing.T) {
	c := collector.NewEnvCollector(collector.EnvConfig{Enabled: false})
	if c.Name() != "env" {
		t.Fatalf("expected name 'env', got %q", c.Name())
	}

	res, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, ok := res.Data.(collector.EnvInfo)
	if !ok {
		t.Fatalf("expected EnvInfo data type, got %T", res.Data)
	}

	if len(info.Captured) != 0 {
		t.Errorf("expected empty captured env vars when disabled, got %d", len(info.Captured))
	}
	if res.Warning == "" {
		t.Errorf("expected privacy warning when disabled")
	}
}

func TestEnvCollector_EnabledWithDenyList(t *testing.T) {
	t.Setenv("REPRO_TEST_SECRET_KEY", "super_secret_val")
	t.Setenv("REPRO_TEST_SAFE_VAR", "safe_val")

	c := collector.NewEnvCollector(collector.EnvConfig{
		Enabled:  true,
		DenyList: []string{"*SECRET*", "*TOKEN*"},
	})

	res, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, ok := res.Data.(collector.EnvInfo)
	if !ok {
		t.Fatalf("expected EnvInfo data type, got %T", res.Data)
	}

	if val, found := info.Captured["REPRO_TEST_SECRET_KEY"]; !found || val != "[REDACTED]" {
		t.Errorf("expected REPRO_TEST_SECRET_KEY to be '[REDACTED]', got %q (found: %v)", val, found)
	}

	if val, found := info.Captured["REPRO_TEST_SAFE_VAR"]; !found || val != "safe_val" {
		t.Errorf("expected REPRO_TEST_SAFE_VAR to be 'safe_val', got %q (found: %v)", val, found)
	}

	if info.RedactedCount < 1 {
		t.Errorf("expected RedactedCount >= 1, got %d", info.RedactedCount)
	}
}

func TestGitCollector_CurrentRepo(t *testing.T) {
	c := collector.NewGitCollector("")
	if c.Name() != "git" {
		t.Fatalf("expected name 'git', got %q", c.Name())
	}

	res, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, ok := res.Data.(collector.GitInfo)
	if !ok {
		t.Fatalf("expected GitInfo data type, got %T", res.Data)
	}

	if !info.Available {
		t.Skip("git not available or not a git repo in workdir; skipping verification of commit")
	}

	if info.Commit == "" {
		t.Errorf("expected non-empty commit hash")
	}
}

func TestGitCollector_NonExistentDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repro_non_git_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	c := collector.NewGitCollector(tempDir)
	res, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error for non-repo: %v", err)
	}

	info, ok := res.Data.(collector.GitInfo)
	if !ok {
		t.Fatalf("expected GitInfo data type, got %T", res.Data)
	}

	if info.Available {
		t.Errorf("expected Available=false for clean temp dir")
	}
	if res.Warning == "" {
		t.Errorf("expected non-empty warning when git is not in a repo")
	}
}
