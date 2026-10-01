package cli

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/internal/config"
	"github.com/BaimPriyatna/repro/src/central"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine"
	"github.com/BaimPriyatna/repro/src/engine/store"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
	graphstore "github.com/BaimPriyatna/repro/src/graph/store"
)

// Shared flag values.
var (
	flagFormat      string
	flagOutput      string
	flagDataDir     string
	flagLocal       bool
	flagSnapshot    string
	flagFrom        string
	flagTo          string
	flagParent      string
	flagMaxDepth    int
	flagAfter       string
	flagBefore      string
	flagLabels      []string
	flagCollectors  []string
	flagReason      string
	flagEmitEvent   bool
	flagMode        string
	flagGraph       string
	flagDeclared    string
	flagObserved    string
	flagEntities    []string
	flagSubject     string
	flagContext     string
	flagResult      string
	flagFileA       string
	flagFileB       string
	flagStrict      bool
	flagAction      string
	flagActiveDays  int
	flagIdleDays    int
	flagDormantDays int
	flagName        string
	flagFromDir     string
	flagYes         bool
)

// ExitCode maps an error into a process exit code.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var reproErr *errors.ReproError
	if stderrors.As(err, &reproErr) {
		switch reproErr.Code {
		case errors.CodeInvalidInput:
			return 2
		case errors.CodeNotFound:
			return 3
		case errors.CodeStorageFailure:
			return 4
		case errors.CodeUnknown, errors.CodeInternal:
			return 1
		default:
			return 1
		}
	}
	return 1
}

// isStandaloneMode reports whether the CLI should operate in local-only mode.
func isStandaloneMode() bool {
	if flagLocal {
		return true
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("REPRO_STORE_MODE")))
	return env == "standalone" || env == "local"
}

// resolveStores provides the Snapshot Store and Event Store for commands.
func resolveStores() (*store.FileStore, event.Store, error) {
	cfg, err := config.Load("")
	if err != nil {
		return nil, nil, err
	}

	dataDir := cfg.Storage.DataDir
	if flagDataDir != "" {
		dataDir = flagDataDir
	}

	if isStandaloneMode() {
		localDir := filepath.Join(dataDir, ".repro")
		if _, err := os.Stat(localDir); os.IsNotExist(err) {
			_ = os.MkdirAll(localDir, 0o750)
		}
		snapStore, err := store.NewFileStore(localDir)
		if err != nil {
			return nil, nil, err
		}
		evStore, err := event.NewFileStore(localDir)
		if err != nil {
			return nil, nil, err
		}
		return snapStore, evStore, nil
	}

	csDir := filepath.Join(dataDir, "store")
	cs, err := central.NewCentralStore(csDir)
	if err != nil {
		return nil, nil, err
	}

	cwd, _ := os.Getwd()
	meta, err := cs.ResolveProject(cwd)
	if err != nil {
		// Fallback to standalone if not yet initialized in central store
		localDir := filepath.Join(cwd, ".repro")
		if _, err := os.Stat(localDir); os.IsNotExist(err) {
			_ = os.MkdirAll(localDir, 0o750)
		}
		snapStore, err := store.NewFileStore(localDir)
		if err != nil {
			return nil, nil, err
		}
		evStore, err := event.NewFileStore(localDir)
		if err != nil {
			return nil, nil, err
		}
		return snapStore, evStore, nil
	}

	snapStore, err := cs.ProjectSnapshotStore(meta.ID)
	if err != nil {
		return nil, nil, err
	}
	evStore, err := cs.ProjectEventStore(meta.ID)
	if err != nil {
		return nil, nil, err
	}

	return snapStore, evStore, nil
}

// resolveSnapshotEngine returns a SnapshotEngine backed by the resolved store.
func resolveSnapshotEngine() (*engine.SnapshotEngine, error) {
	snapStore, _, err := resolveStores()
	if err != nil {
		return nil, err
	}
	return engine.New(snapStore)
}

// renderOutput formats and prints output to stdout or the designated output file.
func renderOutput(cmd *cobra.Command, val any, humanFormatter func() (string, error)) error {
	var outContent string

	if flagFormat == "json" {
		data, err := json.MarshalIndent(val, "", "  ")
		if err != nil {
			return errors.Wrap(errors.CodeInternal, "marshaling JSON output", err)
		}
		outContent = string(data) + "\n"
	} else {
		if humanFormatter != nil {
			formatted, err := humanFormatter()
			if err != nil {
				return err
			}
			outContent = formatted
		} else {
			outContent = defaultHumanFormat(val)
		}
	}

	if flagOutput != "" {
		if err := os.MkdirAll(filepath.Dir(flagOutput), 0o750); err != nil && filepath.Dir(flagOutput) != "." {
			return errors.Wrap(errors.CodeStorageFailure, "creating output directory", err)
		}
		if err := os.WriteFile(flagOutput, []byte(outContent), 0o600); err != nil {
			return errors.Wrap(errors.CodeStorageFailure, "writing output file", err)
		}
		return nil
	}

	_, err := fmt.Fprint(cmd.OutOrStdout(), outContent)
	return err
}

// defaultHumanFormat formats arbitrary domain values into readable text.
func defaultHumanFormat(val any) string {
	switch v := val.(type) {
	case string:
		if !strings.HasSuffix(v, "\n") {
			return v + "\n"
		}
		return v
	case *coreanalysis.AnalysisResult:
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Analysis: %s (Status: %s)\n", v.Analyzer, v.Status))
		if len(v.Findings) == 0 {
			b.WriteString("No findings detected.\n")
		} else {
			b.WriteString(fmt.Sprintf("Findings (%d):\n", len(v.Findings)))
			for _, f := range v.Findings {
				b.WriteString(fmt.Sprintf(" - [%s] [%s] %s: %s\n", f.Severity, f.Type, f.Subject, f.Description))
			}
		}
		return b.String()
	case *snapshot.Snapshot:
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Snapshot ID: %s\n", v.ID))
		b.WriteString(fmt.Sprintf("Timestamp:   %s\n", v.Timestamp.Format("2006-01-02 15:04:05 UTC")))
		b.WriteString(fmt.Sprintf("Source:      %s\n", v.Source))
		b.WriteString(fmt.Sprintf("Hash:        %s\n", v.ContentHash))
		if v.ParentID != "" {
			b.WriteString(fmt.Sprintf("Parent ID:   %s\n", v.ParentID))
		}
		if len(v.Labels) > 0 {
			b.WriteString("Labels:\n")
			for k, val := range v.Labels {
				b.WriteString(fmt.Sprintf("  %s=%s\n", k, val))
			}
		}
		return b.String()
	default:
		data, err := json.MarshalIndent(val, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v\n", val)
		}
		return string(data) + "\n"
	}
}

// parseLabels converts "key=value" string slices into a map.
func parseLabels(raw []string) map[string]string {
	labels := make(map[string]string)
	for _, item := range raw {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			labels[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return labels
}

type rawGraphFile struct {
	Entities  []*entity.Entity     `json:"entities"`
	Relations []*relation.Relation `json:"relations"`
}

// loadGraphFromFile loads entities and relations from a JSON file into a RepairMap.
func loadGraphFromFile(path string) (*repairmap.RepairMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInvalidInput, fmt.Sprintf("reading graph file %q", path), err)
	}

	var g rawGraphFile
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, errors.Wrap(errors.CodeInvalidInput, "unmarshaling graph JSON", err)
	}

	mem := graphstore.NewMemStore()
	for _, e := range g.Entities {
		if err := mem.AddEntity(e); err != nil {
			return nil, err
		}
	}
	for _, r := range g.Relations {
		if err := mem.AddRelation(r); err != nil {
			return nil, err
		}
	}

	return repairmap.New(mem)
}

// resolveSnapshotPair loads two snapshots by ID from store.
func resolveSnapshotPair(ctx context.Context, snapStore store.Store, fromID, toID string) (*snapshot.Snapshot, *snapshot.Snapshot, error) {
	if fromID == "" || toID == "" {
		return nil, nil, errors.New(errors.CodeInvalidInput, "both --from and --to snapshot IDs are required")
	}

	a, err := snapStore.Load(ctx, snapshot.ID(fromID))
	if err != nil {
		return nil, nil, errors.Wrap(errors.CodeNotFound, fmt.Sprintf("loading snapshot %s", fromID), err)
	}

	b, err := snapStore.Load(ctx, snapshot.ID(toID))
	if err != nil {
		return nil, nil, errors.Wrap(errors.CodeNotFound, fmt.Sprintf("loading snapshot %s", toID), err)
	}

	return a, b, nil
}
