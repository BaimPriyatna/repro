package capture_test

import (
	"context"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/capture"
	"github.com/BaimPriyatna/repro/src/capture/repro"
	"github.com/BaimPriyatna/repro/src/capture/timecapsule"
	"github.com/BaimPriyatna/repro/src/capture/watchdog"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine"
	"github.com/BaimPriyatna/repro/src/engine/store"
)

// TestIntegration_AllModulesUseSharedEngine verifies that Repro, TimeCapsule,
// and Watchdog all use the same Snapshot Engine.
func TestIntegration_AllModulesUseSharedEngine(t *testing.T) {
	ctx := context.Background()

	memStore := store.NewMemStore()
	snapshotEngine, err := engine.New(memStore)
	if err != nil {
		t.Fatalf("failed to create snapshot engine: %v", err)
	}

	eventStore := event.NewMemStore()
	eventSystem, err := event.NewSystem(eventStore)
	if err != nil {
		t.Fatalf("failed to create event system: %v", err)
	}

	sharedCfg := &capture.Config{
		Engine:      snapshotEngine,
		EventSystem: eventSystem,
	}

	reproModule, err := repro.New(sharedCfg)
	if err != nil {
		t.Fatalf("failed to create Repro: %v", err)
	}

	tcModule, err := timecapsule.New(&timecapsule.Config{
		Capture: sharedCfg,
	})
	if err != nil {
		t.Fatalf("failed to create TimeCapsule: %v", err)
	}
	defer tcModule.Close()

	wdModule, err := watchdog.New(&watchdog.Config{
		Capture:      sharedCfg,
		AutoSnapshot: true,
	})
	if err != nil {
		t.Fatalf("failed to create Watchdog: %v", err)
	}
	defer wdModule.Close()

	reproResult, err := reproModule.CaptureAndStore(ctx, repro.WithReason("integration test"))
	if err != nil {
		t.Fatalf("Repro.CaptureAndStore() error = %v", err)
	}
	if reproResult.Snapshot == nil {
		t.Fatal("Repro did not create snapshot")
	}
	if reproResult.Snapshot.Source != snapshot.SourceRepro {
		t.Errorf("Repro snapshot source = %v, want %v", reproResult.Snapshot.Source, snapshot.SourceRepro)
	}

	tcResult, err := tcModule.Capture(ctx, timecapsule.WithReason("integration test"))
	if err != nil {
		t.Fatalf("TimeCapsule.Capture() error = %v", err)
	}
	if tcResult.Snapshot == nil {
		t.Fatal("TimeCapsule did not create snapshot")
	}
	if tcResult.Snapshot.Source != snapshot.SourceTimeCapsule {
		t.Errorf("TimeCapsule snapshot source = %v, want %v", tcResult.Snapshot.Source, snapshot.SourceTimeCapsule)
	}

	change := &watchdog.Change{
		Type:      watchdog.ChangeTypeFileModified,
		Subject:   "integration-test.go",
		Timestamp: time.Now(),
	}
	wdResult, err := wdModule.ReportChange(ctx, change)
	if err != nil {
		t.Fatalf("Watchdog.ReportChange() error = %v", err)
	}
	if wdResult.Snapshot == nil {
		t.Fatal("Watchdog did not create snapshot")
	}
	if wdResult.Snapshot.Source != snapshot.SourceWatchdog {
		t.Errorf("Watchdog snapshot source = %v, want %v", wdResult.Snapshot.Source, snapshot.SourceWatchdog)
	}

	allSnapshots, err := snapshotEngine.List(ctx, store.Filter{})
	if err != nil {
		t.Fatalf("failed to list snapshots: %v", err)
	}

	if len(allSnapshots) != 3 {
		t.Fatalf("expected 3 snapshots in shared store, got %d", len(allSnapshots))
	}

	for _, snap := range []*snapshot.Snapshot{
		reproResult.Snapshot,
		tcResult.Snapshot,
		wdResult.Snapshot,
	} {
		loaded, err := snapshotEngine.Load(ctx, snap.ID)
		if err != nil {
			t.Errorf("failed to load snapshot %s: %v", snap.ID, err)
		}
		if loaded.ID != snap.ID {
			t.Errorf("loaded snapshot ID mismatch: got %s, want %s", loaded.ID, snap.ID)
		}
	}

	t.Logf("all three modules used the same Snapshot Engine")
	t.Logf("  - Repro snapshot: %s", reproResult.Snapshot.ID)
	t.Logf("  - TimeCapsule snapshot: %s", tcResult.Snapshot.ID)
	t.Logf("  - Watchdog snapshot: %s", wdResult.Snapshot.ID)
}

// TestIntegration_EventFlow verifies the Watchdog -> Event -> Snapshot flow.
func TestIntegration_EventFlow(t *testing.T) {
	ctx := context.Background()

	memStore := store.NewMemStore()
	snapshotEngine, err := engine.New(memStore)
	if err != nil {
		t.Fatalf("failed to create snapshot engine: %v", err)
	}

	eventStore := event.NewMemStore()
	eventSystem, err := event.NewSystem(eventStore)
	if err != nil {
		t.Fatalf("failed to create event system: %v", err)
	}

	// Create Watchdog with auto-snapshot
	wd, err := watchdog.New(&watchdog.Config{
		Capture: &capture.Config{
			Engine:      snapshotEngine,
			EventSystem: eventSystem,
		},
		AutoSnapshot: true,
	})
	if err != nil {
		t.Fatalf("failed to create Watchdog: %v", err)
	}
	defer wd.Close()

	// Report a change
	change := &watchdog.Change{
		Type:      watchdog.ChangeTypeConfigChanged,
		Subject:   "app.config",
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"key": "test",
		},
	}

	result, err := wd.ReportChange(ctx, change)
	if err != nil {
		t.Fatalf("ReportChange() error = %v", err)
	}

	// Verify Event was emitted
	if len(result.Events) < 1 {
		t.Fatal("no events emitted")
	}
	changeEvent := result.Events[0]
	if changeEvent.Source != event.SourceWatchdog {
		t.Errorf("event source = %v, want %v", changeEvent.Source, event.SourceWatchdog)
	}

	// Verify Snapshot was created
	if result.Snapshot == nil {
		t.Fatal("snapshot was not created")
	}

	// Verify the event is queryable from the event system
	events, err := eventSystem.Query(ctx, event.Query{
		Source: event.SourceWatchdog,
	})
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}
	if len(events) < 1 {
		t.Fatal("no watchdog events found in event system")
	}

	// Verify the snapshot is retrievable
	loaded, err := snapshotEngine.Load(ctx, result.Snapshot.ID)
	if err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}
	if loaded.ID != result.Snapshot.ID {
		t.Errorf("loaded snapshot ID mismatch")
	}

	t.Logf("verified Watchdog -> Event -> Snapshot")
	t.Logf("  - Event ID: %s (type: %s)", changeEvent.ID, changeEvent.Type)
	t.Logf("  - Snapshot ID: %s", result.Snapshot.ID)
}

// TestIntegration_HistoricalLinkage verifies snapshots can be linked via parent references.
func TestIntegration_HistoricalLinkage(t *testing.T) {
	ctx := context.Background()

	memStore := store.NewMemStore()
	snapshotEngine, err := engine.New(memStore)
	if err != nil {
		t.Fatalf("failed to create snapshot engine: %v", err)
	}

	eventStore := event.NewMemStore()
	eventSystem, err := event.NewSystem(eventStore)
	if err != nil {
		t.Fatalf("failed to create event system: %v", err)
	}

	// Create TimeCapsule
	tc, err := timecapsule.New(&timecapsule.Config{
		Capture: &capture.Config{
			Engine:      snapshotEngine,
			EventSystem: eventSystem,
		},
	})
	if err != nil {
		t.Fatalf("failed to create TimeCapsule: %v", err)
	}
	defer tc.Close()

	// Create first snapshot
	result1, err := tc.Capture(ctx, timecapsule.WithReason("baseline"))
	if err != nil {
		t.Fatalf("first Capture() error = %v", err)
	}

	// Create second snapshot (should auto-link to first)
	result2, err := tc.Capture(ctx, timecapsule.WithReason("update"))
	if err != nil {
		t.Fatalf("second Capture() error = %v", err)
	}

	// Verify linkage
	if result2.Snapshot.ParentID != result1.Snapshot.ID {
		t.Errorf("snapshot 2 ParentID = %v, want %v", result2.Snapshot.ParentID, result1.Snapshot.ID)
	}

	// Create third snapshot with explicit parent
	explicitParentID := result1.Snapshot.ID
	result3, err := tc.Capture(ctx, timecapsule.WithParent(explicitParentID), timecapsule.WithReason("branch"))
	if err != nil {
		t.Fatalf("third Capture() error = %v", err)
	}

	if result3.Snapshot.ParentID != explicitParentID {
		t.Errorf("snapshot 3 ParentID = %v, want %v", result3.Snapshot.ParentID, explicitParentID)
	}

	t.Logf("✓ Verified historical linkage via parent references")
	t.Logf("  - Snapshot 1: %s (no parent)", result1.Snapshot.ID)
	t.Logf("  - Snapshot 2: %s (parent: %s)", result2.Snapshot.ID, result2.Snapshot.ParentID)
	t.Logf("  - Snapshot 3: %s (parent: %s)", result3.Snapshot.ID, result3.Snapshot.ParentID)
}
