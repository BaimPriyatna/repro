package watchdog_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/capture"
	"github.com/BaimPriyatna/repro/src/capture/watchdog"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine"
)

// mockEngine implements capture.Engine for testing.
type mockEngine struct {
	mu          sync.Mutex
	captureFunc func(ctx context.Context, source snapshot.Source, opts ...engine.CaptureOption) (*snapshot.Snapshot, error)
	storeFunc   func(ctx context.Context, snap *snapshot.Snapshot) error
	loadFunc    func(ctx context.Context, id snapshot.ID) (*snapshot.Snapshot, error)
	captured    []*snapshot.Snapshot
	stored      []*snapshot.Snapshot
}

func (m *mockEngine) Capture(ctx context.Context, source snapshot.Source, opts ...engine.CaptureOption) (*snapshot.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.captureFunc != nil {
		return m.captureFunc(ctx, source, opts...)
	}
	snap := &snapshot.Snapshot{
		ID:        snapshot.NewID(),
		Timestamp: time.Now().UTC(),
		Source:    source,
		Data:      map[string]any{"mock": "data"},
	}
	m.captured = append(m.captured, snap)
	return snap, nil
}

func (m *mockEngine) Store(ctx context.Context, snap *snapshot.Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.storeFunc != nil {
		return m.storeFunc(ctx, snap)
	}
	m.stored = append(m.stored, snap)
	return nil
}

func (m *mockEngine) Load(ctx context.Context, id snapshot.ID) (*snapshot.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.loadFunc != nil {
		return m.loadFunc(ctx, id)
	}
	return &snapshot.Snapshot{ID: id}, nil
}

func (m *mockEngine) captureCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.captured)
}

func (m *mockEngine) storeCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.stored)
}

// mockEventSystem implements capture.EventSystem for testing.
type mockEventSystem struct {
	mu         sync.Mutex
	emitFunc   func(ctx context.Context, eventType event.Type, source event.Source, subject string, opts ...event.Option) (*event.Event, error)
	recordFunc func(ctx context.Context, evt *event.Event) error
	emitted    []*event.Event
}

func (m *mockEventSystem) Emit(ctx context.Context, eventType event.Type, source event.Source, subject string, opts ...event.Option) (*event.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.emitFunc != nil {
		return m.emitFunc(ctx, eventType, source, subject, opts...)
	}
	evt, err := event.New(eventType, source, subject, opts...)
	if err != nil {
		return nil, err
	}
	m.emitted = append(m.emitted, evt)
	return evt, nil
}

func (m *mockEventSystem) Record(ctx context.Context, evt *event.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.recordFunc != nil {
		return m.recordFunc(ctx, evt)
	}
	m.emitted = append(m.emitted, evt)
	return nil
}

func (m *mockEventSystem) emitCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.emitted)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *watchdog.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &watchdog.Config{
				Capture: &capture.Config{
					Engine:      &mockEngine{},
					EventSystem: &mockEventSystem{},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with auto-snapshot",
			config: &watchdog.Config{
				Capture: &capture.Config{
					Engine:      &mockEngine{},
					EventSystem: &mockEventSystem{},
				},
				AutoSnapshot: true,
			},
			wantErr: false,
		},
		{
			name: "missing capture config",
			config: &watchdog.Config{
				AutoSnapshot: true,
			},
			wantErr: true,
		},
		{
			name: "missing engine",
			config: &watchdog.Config{
				Capture: &capture.Config{
					EventSystem: &mockEventSystem{},
				},
			},
			wantErr: true,
		},
		{
			name: "missing event system",
			config: &watchdog.Config{
				Capture: &capture.Config{
					Engine: &mockEngine{},
				},
			},
			wantErr: true,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd, err := watchdog.New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && wd == nil {
				t.Error("New() returned nil Watchdog for valid config")
			}
			if wd != nil {
				_ = wd.Close()
			}
		})
	}
}

func TestWatchdog_ReportChange(t *testing.T) {
	t.Run("report change emits event", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &watchdog.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
			AutoSnapshot: false,
		}

		wd, err := watchdog.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer wd.Close()

		ctx := context.Background()
		change := &watchdog.Change{
			Type:      watchdog.ChangeTypeFileModified,
			Subject:   "test.go",
			Timestamp: time.Now(),
		}

		result, err := wd.ReportChange(ctx, change)
		if err != nil {
			t.Errorf("ReportChange() error = %v", err)
		}
		if result == nil {
			t.Fatal("ReportChange() returned nil result")
		}
		if len(result.Events) != 1 {
			t.Errorf("result has %d events, want 1", len(result.Events))
		}
		if evtSys.emitCount() != 1 {
			t.Errorf("event system emitted %d events, want 1", evtSys.emitCount())
		}
		if result.Snapshot != nil {
			t.Error("result should not have snapshot (AutoSnapshot=false)")
		}
	})

	t.Run("report change with auto-snapshot", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &watchdog.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
			AutoSnapshot: true,
		}

		wd, err := watchdog.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer wd.Close()

		ctx := context.Background()
		change := &watchdog.Change{
			Type:      watchdog.ChangeTypeFileModified,
			Subject:   "test.go",
			Timestamp: time.Now(),
		}

		result, err := wd.ReportChange(ctx, change)
		if err != nil {
			t.Errorf("ReportChange() error = %v", err)
		}
		if result == nil {
			t.Fatal("ReportChange() returned nil result")
		}
		if result.Snapshot == nil {
			t.Error("result should have snapshot (AutoSnapshot=true)")
		}
		if result.Snapshot.Source != snapshot.SourceWatchdog {
			t.Errorf("snapshot.Source = %v, want %v", result.Snapshot.Source, snapshot.SourceWatchdog)
		}
		// Should emit 2 events: change event + snapshot created event
		if len(result.Events) != 2 {
			t.Errorf("result has %d events, want 2", len(result.Events))
		}
		if eng.captureCount() != 1 {
			t.Errorf("engine captured %d snapshots, want 1", eng.captureCount())
		}
		if eng.storeCount() != 1 {
			t.Errorf("engine stored %d snapshots, want 1", eng.storeCount())
		}
	})

	t.Run("report change with throttling", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &watchdog.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
			AutoSnapshot: true,
			Throttle:     1 * time.Second,
		}

		wd, err := watchdog.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer wd.Close()

		ctx := context.Background()
		change := &watchdog.Change{
			Type:      watchdog.ChangeTypeFileModified,
			Subject:   "test.go",
			Timestamp: time.Now(),
		}

		// First change should trigger snapshot
		result1, err := wd.ReportChange(ctx, change)
		if err != nil {
			t.Errorf("ReportChange() error = %v", err)
		}
		if result1.Snapshot == nil {
			t.Error("first change should trigger snapshot")
		}

		// Second change immediately after should NOT trigger snapshot (throttled)
		result2, err := wd.ReportChange(ctx, change)
		if err != nil {
			t.Errorf("ReportChange() error = %v", err)
		}
		if result2.Snapshot != nil {
			t.Error("second change should be throttled")
		}

		// Should have 1 snapshot total (not 2)
		if eng.captureCount() != 1 {
			t.Errorf("engine captured %d snapshots, want 1 (throttled)", eng.captureCount())
		}
	})

	t.Run("report change with handler", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}

		handlerCalled := false
		handler := func(ctx context.Context, change *watchdog.Change) (bool, error) {
			handlerCalled = true
			return true, nil // trigger snapshot
		}

		cfg := &watchdog.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
			AutoSnapshot: false,
			Handlers:     []watchdog.ChangeHandler{handler},
		}

		wd, err := watchdog.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer wd.Close()

		ctx := context.Background()
		change := &watchdog.Change{
			Type:      watchdog.ChangeTypeFileModified,
			Subject:   "test.go",
			Timestamp: time.Now(),
		}

		result, err := wd.ReportChange(ctx, change)
		if err != nil {
			t.Errorf("ReportChange() error = %v", err)
		}
		if !handlerCalled {
			t.Error("handler was not called")
		}
		if result.Snapshot == nil {
			t.Error("handler should have triggered snapshot")
		}
	})

	t.Run("report nil change", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &watchdog.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
		}

		wd, err := watchdog.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer wd.Close()

		ctx := context.Background()
		_, err = wd.ReportChange(ctx, nil)
		if err == nil {
			t.Error("ReportChange(nil) should return error")
		}
	})
}

func TestWatchdog_StartStop(t *testing.T) {
	t.Run("start and stop", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &watchdog.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
		}

		wd, err := watchdog.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer wd.Close()

		if wd.IsRunning() {
			t.Error("Watchdog should not be running initially")
		}

		err = wd.Start()
		if err != nil {
			t.Errorf("Start() error = %v", err)
		}

		if !wd.IsRunning() {
			t.Error("Watchdog should be running after Start()")
		}

		err = wd.Stop()
		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}

		if wd.IsRunning() {
			t.Error("Watchdog should not be running after Stop()")
		}
	})

	t.Run("start already running", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &watchdog.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
		}

		wd, err := watchdog.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer wd.Close()

		err = wd.Start()
		if err != nil {
			t.Errorf("Start() error = %v", err)
		}

		err = wd.Start()
		if err == nil {
			t.Error("Start() on already running Watchdog should return error")
		}

		_ = wd.Stop()
	})

	t.Run("stop not running", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &watchdog.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
		}

		wd, err := watchdog.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer wd.Close()

		err = wd.Stop()
		if err == nil {
			t.Error("Stop() on non-running Watchdog should return error")
		}
	})
}

func TestWatchdog_AutoStart(t *testing.T) {
	eng := &mockEngine{}
	evtSys := &mockEventSystem{}
	cfg := &watchdog.Config{
		Capture: &capture.Config{
			Engine:      eng,
			EventSystem: evtSys,
		},
		AutoStart: true,
	}

	wd, err := watchdog.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wd.Close()

	if !wd.IsRunning() {
		t.Error("Watchdog should be running with AutoStart=true")
	}
}

func TestWatchdog_AddHandler(t *testing.T) {
	eng := &mockEngine{}
	evtSys := &mockEventSystem{}
	cfg := &watchdog.Config{
		Capture: &capture.Config{
			Engine:      eng,
			EventSystem: evtSys,
		},
	}

	wd, err := watchdog.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wd.Close()

	handlerCalled := false
	handler := func(ctx context.Context, change *watchdog.Change) (bool, error) {
		handlerCalled = true
		return false, nil
	}

	wd.AddHandler(handler)

	ctx := context.Background()
	change := &watchdog.Change{
		Type:      watchdog.ChangeTypeFileModified,
		Subject:   "test.go",
		Timestamp: time.Now(),
	}

	_, err = wd.ReportChange(ctx, change)
	if err != nil {
		t.Errorf("ReportChange() error = %v", err)
	}

	if !handlerCalled {
		t.Error("added handler was not called")
	}
}

func TestWatchdog_Close(t *testing.T) {
	t.Run("close running", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &watchdog.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
		}

		wd, err := watchdog.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		err = wd.Start()
		if err != nil {
			t.Errorf("Start() error = %v", err)
		}

		err = wd.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}

		if wd.IsRunning() {
			t.Error("Watchdog should not be running after Close()")
		}
	})

	t.Run("close not running", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &watchdog.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
		}

		wd, err := watchdog.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		err = wd.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

func TestWatchdog_CanonicalFlow(t *testing.T) {
	eng := &mockEngine{}
	evtSys := &mockEventSystem{}
	cfg := &watchdog.Config{
		Capture: &capture.Config{
			Engine:      eng,
			EventSystem: evtSys,
		},
		AutoSnapshot: true,
	}

	wd, err := watchdog.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wd.Close()

	ctx := context.Background()
	change := &watchdog.Change{
		Type:      watchdog.ChangeTypeConfigChanged,
		Subject:   "config.toml",
		Timestamp: time.Now(),
	}

	result, err := wd.ReportChange(ctx, change)
	if err != nil {
		t.Errorf("ReportChange() error = %v", err)
	}

	if evtSys.emitCount() < 1 {
		t.Error("no events emitted")
	}

	if result.Snapshot == nil {
		t.Error("snapshot was not created")
	}
	if result.Snapshot.Source != snapshot.SourceWatchdog {
		t.Errorf("snapshot.Source = %v, want %v", result.Snapshot.Source, snapshot.SourceWatchdog)
	}
	if eng.captureCount() != 1 {
		t.Errorf("engine captured %d snapshots, want 1", eng.captureCount())
	}

	if len(result.Events) < 1 {
		t.Error("no events in result")
	}
}

func TestWatchdog_UsesSharedEngine(t *testing.T) {
	eng := &mockEngine{}
	evtSys := &mockEventSystem{}
	cfg := &watchdog.Config{
		Capture: &capture.Config{
			Engine:      eng,
			EventSystem: evtSys,
		},
		AutoSnapshot: true,
	}

	wd, err := watchdog.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wd.Close()

	ctx := context.Background()
	change := &watchdog.Change{
		Type:      watchdog.ChangeTypeFileModified,
		Subject:   "test.go",
		Timestamp: time.Now(),
	}

	// Report multiple changes
	_, err = wd.ReportChange(ctx, change)
	if err != nil {
		t.Errorf("ReportChange() error = %v", err)
	}

	// Wait for throttle to pass
	time.Sleep(10 * time.Millisecond)

	_, err = wd.ReportChange(ctx, change)
	if err != nil {
		t.Errorf("ReportChange() error = %v", err)
	}

	// Verify all snapshots went through the same engine
	if eng.captureCount() != 2 {
		t.Errorf("engine captured %d snapshots, want 2", eng.captureCount())
	}

	// Verify all snapshots have the correct source
	eng.mu.Lock()
	defer eng.mu.Unlock()
	for i, snap := range eng.captured {
		if snap.Source != snapshot.SourceWatchdog {
			t.Errorf("snapshot[%d].Source = %v, want %v", i, snap.Source, snapshot.SourceWatchdog)
		}
	}
}
