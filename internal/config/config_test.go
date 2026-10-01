package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BaimPriyatna/repro/internal/config"
)

func TestDefaults_DataDirNotEmpty(t *testing.T) {
	cfg := config.Defaults()
	if cfg.Storage.DataDir == "" {
		t.Error("default DataDir must not be empty")
	}
}

func TestDefaults_PrivacyCaptureEnvVarsFalse(t *testing.T) {
	cfg := config.Defaults()
	if cfg.Privacy.CaptureEnvVars {
		t.Error("CaptureEnvVars must default to false (secrets must not be captured by default)")
	}
}

func TestDefaults_DenyListNotEmpty(t *testing.T) {
	cfg := config.Defaults()
	if len(cfg.Privacy.EnvVarDenyList) == 0 {
		t.Error("default EnvVarDenyList must contain patterns for common secret env vars")
	}
}

func TestDefaults_LogLevelInfo(t *testing.T) {
	cfg := config.Defaults()
	if cfg.Logging.Level != config.LogLevelInfo {
		t.Errorf("default LogLevel = %q; want %q", cfg.Logging.Level, config.LogLevelInfo)
	}
}

func TestLoad_EmptyPathReturnsDefaults(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load(\"\") returned error: %v", err)
	}
	if cfg.Storage.DataDir == "" {
		t.Error("Load with empty path should return defaults with non-empty DataDir")
	}
}

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	cfg, err := config.Load("/non/existent/path/config.toml")
	if err != nil {
		t.Fatalf("Load with missing file should not error, got: %v", err)
	}
	if cfg.Storage.DataDir == "" {
		t.Error("Load with missing file should return defaults")
	}
}

func TestLoad_ValidTOML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
[storage]
data_dir = "/custom/data"

[logging]
level = "debug"

[privacy]
capture_env_vars = false
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Storage.DataDir != "/custom/data" {
		t.Errorf("DataDir = %q; want /custom/data", cfg.Storage.DataDir)
	}
	if cfg.Logging.Level != config.LogLevelDebug {
		t.Errorf("Level = %q; want debug", cfg.Logging.Level)
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(cfgPath, []byte("[[[[not valid"), 0o600); err != nil {
		t.Fatalf("failed to write bad config: %v", err)
	}
	_, err := config.Load(cfgPath)
	if err == nil {
		t.Error("Load with invalid TOML should return an error")
	}
}
