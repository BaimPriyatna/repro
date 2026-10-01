// Package config loads TOML configuration and applies environment overrides.
// Secrets must not be captured by default.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// DefaultConfigFileName is the filename Repro looks for in the config directory.
const DefaultConfigFileName = "config.toml"

// LogLevel controls verbosity of the application logger.
type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
)

// Config is the root configuration for the Repro system.
// All fields have safe defaults via [Defaults].
type Config struct {
	// Storage configures where Repro persists snapshots and events.
	Storage StorageConfig `toml:"storage"`

	// Logging configures the application logger.
	Logging LoggingConfig `toml:"logging"`

	// Privacy controls what is and is not captured.
	Privacy PrivacyConfig `toml:"privacy"`
}

// StorageConfig holds persistence settings.
type StorageConfig struct {
	// DataDir is the root directory for all Repro data.
	// Defaults to $HOME/.repro
	DataDir string `toml:"data_dir"`
}

// LoggingConfig holds logger settings.
type LoggingConfig struct {
	// Level sets the minimum log level. One of: debug, info, warning, error.
	Level LogLevel `toml:"level"`
}

// PrivacyConfig controls sensitive-data capture behaviour.
type PrivacyConfig struct {
	// CaptureEnvVars controls whether environment variables are captured.
	// Defaults to false — secrets must not be captured by default.
	CaptureEnvVars bool `toml:"capture_env_vars"`

	// EnvVarDenyList is a list of env-var name patterns that are always
	// redacted even when CaptureEnvVars is true.
	EnvVarDenyList []string `toml:"env_var_deny_list"`
}

// Defaults returns a Config populated with safe, zero-config defaults.
func Defaults() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Storage: StorageConfig{
			DataDir: filepath.Join(home, ".repro"),
		},
		Logging: LoggingConfig{
			Level: LogLevelInfo,
		},
		Privacy: PrivacyConfig{
			CaptureEnvVars: false,
			EnvVarDenyList: []string{
				"*KEY*", "*SECRET*", "*TOKEN*", "*PASSWORD*", "*PASSWD*",
				"*CREDENTIAL*", "*PRIVATE*",
			},
		},
	}
}

// Load reads a TOML config file from path and merges it over [Defaults].
// If path is empty, only the defaults are returned (valid zero-config usage).
// It returns an error if the file exists but cannot be parsed.
func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		return cfg, nil
	}

	f, err := os.Open(path) //nolint:gosec // path is caller-supplied intentionally
	if err != nil {
		if os.IsNotExist(err) {
			// Missing file → use defaults silently.
			return cfg, nil
		}
		return cfg, fmt.Errorf("opening config file %q: %w", path, err)
	}
	defer f.Close()

	if _, err := toml.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	return cfg, nil
}

// DefaultConfigPath returns the canonical config-file path for the current user.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".repro", DefaultConfigFileName)
}
