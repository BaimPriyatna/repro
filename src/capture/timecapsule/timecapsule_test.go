package timecapsule_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/capture"
	"github.com/BaimPriyatna/repro/src/capture/timecapsule"
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
		config  *timecapsule.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &timecapsule.Config{
				Capture: &capture.Config{
					Engine: &mockEngine{},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with interval",
			config: &timecapsule.Config{
				Capture: &capture.Config{
					Engine: &mockEngine{},
				},
				Interval: 5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "missing capture config",
			config: &timecapsule.Config{
				Interval: 5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "missing engine",
			config: &timecapsule.Config{
				Capture: &capture.Config{},
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
			tc, err := timecapsule.New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tc == nil {
				t.Error("New() returned nil TimeCapsule for valid config")
			}
			if tc != nil {
				_ = tc.Close()
			}
		})
	}
}

func TestTimeCapsule_Capture(t *testing.T) {
	t.Run("basic capture", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer tc.Close()

		ctx := context.Background()
		result, err := tc.Capture(ctx)

		if err != nil {
			t.Errorf("Capture() error = %v", err)
		}
		if result == nil {
			t.Fatal("Capture() returned nil result")
		}
		if result.Snapshot == nil {
			t.Error("Capture() result has nil Snapshot")
		}
		if result.Snapshot.Source != snapshot.SourceTimeCapsule {
			t.Errorf("Snapshot.Source = %v, want %v", result.Snapshot.Source, snapshot.SourceTimeCapsule)
		}
		if eng.captureCount() != 1 {
			t.Errorf("engine captured %d snapshots, want 1", eng.captureCount())
		}
		if eng.storeCount() != 1 {
			t.Errorf("engine stored %d snapshots, want 1", eng.storeCount())
		}
	})

	t.Run("capture with reason", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer tc.Close()

		ctx := context.Background()
		result, err := tc.Capture(ctx, timecapsule.WithReason("testing"))

		if err != nil {
			t.Errorf("Capture() error = %v", err)
		}
		if result == nil {
			t.Fatal("Capture() returned nil result")
		}
		if result.Snapshot == nil {
			t.Error("Capture() result has nil Snapshot")
		}
	})

	t.Run("capture with labels", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer tc.Close()

		ctx := context.Background()
		labels := map[string]string{
			"env":  "test",
			"user": "alice",
		}
		result, err := tc.Capture(ctx, timecapsule.WithLabels(labels))

		if err != nil {
			t.Errorf("Capture() error = %v", err)
		}
		if result == nil {
			t.Fatal("Capture() returned nil result")
		}
	})

	t.Run("capture with event emission", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine:      eng,
				EventSystem: evtSys,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer tc.Close()

		ctx := context.Background()
		result, err := tc.Capture(ctx, timecapsule.WithReason("test"))

		if err != nil {
			t.Errorf("Capture() error = %v", err)
		}
		if result == nil {
			t.Fatal("Capture() returned nil result")
		}
		if len(result.Events) != 1 {
			t.Errorf("result has %d events, want 1", len(result.Events))
		}
		if evtSys.emitCount() != 1 {
			t.Errorf("event system emitted %d events, want 1", evtSys.emitCount())
		}
	})

	t.Run("multiple captures link via parent", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer tc.Close()

		ctx := context.Background()

		// First capture
		result1, err := tc.Capture(ctx)
		if err != nil {
			t.Fatalf("Capture() error = %v", err)
		}

		// LastCaptureID should be set
		lastID := tc.LastCaptureID()
		if lastID == "" {
			t.Error("LastCaptureID() returned empty after first capture")
		}
		if lastID != result1.Snapshot.ID {
			t.Errorf("LastCaptureID() = %v, want %v", lastID, result1.Snapshot.ID)
		}

		// Second capture (should auto-link to first)
		result2, err := tc.Capture(ctx)
		if err != nil {
			t.Fatalf("Capture() error = %v", err)
		}

		// LastCaptureID should be updated
		lastID = tc.LastCaptureID()
		if lastID != result2.Snapshot.ID {
			t.Errorf("LastCaptureID() = %v, want %v", lastID, result2.Snapshot.ID)
		}
	})
}

func TestTimeCapsule_StartStop(t *testing.T) {
	t.Run("start and stop", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer tc.Close()

		if tc.IsRunning() {
			t.Error("TimeCapsule should not be running initially")
		}

		err = tc.Start(100 * time.Millisecond)
		if err != nil {
			t.Errorf("Start() error = %v", err)
		}

		if !tc.IsRunning() {
			t.Error("TimeCapsule should be running after Start()")
		}

		err = tc.Stop()
		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}

		if tc.IsRunning() {
			t.Error("TimeCapsule should not be running after Stop()")
		}
	})

	t.Run("start with invalid interval", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer tc.Close()

		err = tc.Start(0)
		if err == nil {
			t.Error("Start(0) should return error")
		}

		err = tc.Start(-1 * time.Second)
		if err == nil {
			t.Error("Start(-1s) should return error")
		}
	})

	t.Run("start already running", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer tc.Close()

		err = tc.Start(100 * time.Millisecond)
		if err != nil {
			t.Errorf("Start() error = %v", err)
		}

		err = tc.Start(100 * time.Millisecond)
		if err == nil {
			t.Error("Start() on already running TimeCapsule should return error")
		}

		_ = tc.Stop()
	})

	t.Run("stop not running", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer tc.Close()

		err = tc.Stop()
		if err == nil {
			t.Error("Stop() on non-running TimeCapsule should return error")
		}
	})

	t.Run("periodic capture", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer tc.Close()

		// Start with short interval for testing
		err = tc.Start(50 * time.Millisecond)
		if err != nil {
			t.Errorf("Start() error = %v", err)
		}

		// Wait for at least 2 captures
		time.Sleep(150 * time.Millisecond)

		err = tc.Stop()
		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}

		// Should have captured at least 2 snapshots
		if eng.captureCount() < 2 {
			t.Errorf("engine captured %d snapshots, want at least 2", eng.captureCount())
		}
	})
}

func TestTimeCapsule_AutoStart(t *testing.T) {
	eng := &mockEngine{}
	cfg := &timecapsule.Config{
		Capture: &capture.Config{
			Engine: eng,
		},
		Interval:  50 * time.Millisecond,
		AutoStart: true,
	}

	tc, err := timecapsule.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer tc.Close()

	if !tc.IsRunning() {
		t.Error("TimeCapsule should be running with AutoStart=true")
	}

	// Wait for at least one capture
	time.Sleep(100 * time.Millisecond)

	if eng.captureCount() < 1 {
		t.Errorf("engine captured %d snapshots, want at least 1", eng.captureCount())
	}
}

func TestTimeCapsule_Close(t *testing.T) {
	t.Run("close running", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		err = tc.Start(100 * time.Millisecond)
		if err != nil {
			t.Errorf("Start() error = %v", err)
		}

		err = tc.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}

		if tc.IsRunning() {
			t.Error("TimeCapsule should not be running after Close()")
		}
	})

	t.Run("close not running", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &timecapsule.Config{
			Capture: &capture.Config{
				Engine: eng,
			},
		}

		tc, err := timecapsule.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		err = tc.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

func TestTimeCapsule_UsesSharedEngine(t *testing.T) {
	eng := &mockEngine{}
	cfg := &timecapsule.Config{
		Capture: &capture.Config{
			Engine: eng,
		},
	}

	tc, err := timecapsule.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer tc.Close()

	ctx := context.Background()

	// Capture multiple times
	_, err = tc.Capture(ctx)
	if err != nil {
		t.Errorf("Capture() error = %v", err)
	}

	_, err = tc.Capture(ctx)
	if err != nil {
		t.Errorf("Capture() error = %v", err)
	}

	// Verify both captures went through the same engine
	if eng.captureCount() != 2 {
		t.Errorf("engine captured %d snapshots, want 2", eng.captureCount())
	}

	// Verify all snapshots have the correct source
	eng.mu.Lock()
	defer eng.mu.Unlock()
	for i, snap := range eng.captured {
		if snap.Source != snapshot.SourceTimeCapsule {
			t.Errorf("snapshot[%d].Source = %v, want %v", i, snap.Source, snapshot.SourceTimeCapsule)
		}
	}
}
