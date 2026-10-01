package repro_test

import (
	"context"
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/capture"
	"github.com/BaimPriyatna/repro/src/capture/repro"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine"
)

// mockEngine implements capture.Engine for testing.
type mockEngine struct {
	captureFunc func(ctx context.Context, source snapshot.Source, opts ...engine.CaptureOption) (*snapshot.Snapshot, error)
	storeFunc   func(ctx context.Context, snap *snapshot.Snapshot) error
	loadFunc    func(ctx context.Context, id snapshot.ID) (*snapshot.Snapshot, error)
	captured    []*snapshot.Snapshot
	stored      []*snapshot.Snapshot
}

func (m *mockEngine) Capture(ctx context.Context, source snapshot.Source, opts ...engine.CaptureOption) (*snapshot.Snapshot, error) {
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
	if m.storeFunc != nil {
		return m.storeFunc(ctx, snap)
	}
	m.stored = append(m.stored, snap)
	return nil
}

func (m *mockEngine) Load(ctx context.Context, id snapshot.ID) (*snapshot.Snapshot, error) {
	if m.loadFunc != nil {
		return m.loadFunc(ctx, id)
	}
	return &snapshot.Snapshot{ID: id}, nil
}

// mockEventSystem implements capture.EventSystem for testing.
type mockEventSystem struct {
	emitFunc   func(ctx context.Context, eventType event.Type, source event.Source, subject string, opts ...event.Option) (*event.Event, error)
	recordFunc func(ctx context.Context, evt *event.Event) error
	emitted    []*event.Event
}

func (m *mockEventSystem) Emit(ctx context.Context, eventType event.Type, source event.Source, subject string, opts ...event.Option) (*event.Event, error) {
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
	if m.recordFunc != nil {
		return m.recordFunc(ctx, evt)
	}
	m.emitted = append(m.emitted, evt)
	return nil
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *capture.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &capture.Config{
				Engine: &mockEngine{},
			},
			wantErr: false,
		},
		{
			name: "valid config with event system",
			config: &capture.Config{
				Engine:      &mockEngine{},
				EventSystem: &mockEventSystem{},
			},
			wantErr: false,
		},
		{
			name: "missing engine",
			config: &capture.Config{
				EventSystem: &mockEventSystem{},
			},
			wantErr: true,
		},
		{
			name:    "nil config",
			config:  &capture.Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := repro.New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && r == nil {
				t.Error("New() returned nil Repro for valid config")
			}
		})
	}
}

func TestRepro_Capture(t *testing.T) {
	t.Run("basic capture", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &capture.Config{
			Engine: eng,
		}

		r, err := repro.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		ctx := context.Background()
		result, err := r.Capture(ctx)

		if err != nil {
			t.Errorf("Capture() error = %v", err)
		}
		if result == nil {
			t.Fatal("Capture() returned nil result")
		}
		if result.Snapshot == nil {
			t.Error("Capture() result has nil Snapshot")
		}
		if result.Snapshot.Source != snapshot.SourceRepro {
			t.Errorf("Snapshot.Source = %v, want %v", result.Snapshot.Source, snapshot.SourceRepro)
		}
		if len(eng.captured) != 1 {
			t.Errorf("engine captured %d snapshots, want 1", len(eng.captured))
		}
	})

	t.Run("capture with reason", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &capture.Config{
			Engine: eng,
		}

		r, err := repro.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		ctx := context.Background()
		result, err := r.Capture(ctx, repro.WithReason("testing"))

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
		cfg := &capture.Config{
			Engine: eng,
		}

		r, err := repro.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		ctx := context.Background()
		labels := map[string]string{
			"env":  "test",
			"user": "alice",
		}
		result, err := r.Capture(ctx, repro.WithLabels(labels))

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

	t.Run("capture with parent", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &capture.Config{
			Engine: eng,
		}

		r, err := repro.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		ctx := context.Background()
		parentID := snapshot.NewID()
		result, err := r.Capture(ctx, repro.WithParent(parentID))

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

	t.Run("capture with event emission", func(t *testing.T) {
		eng := &mockEngine{}
		evtSys := &mockEventSystem{}
		cfg := &capture.Config{
			Engine:      eng,
			EventSystem: evtSys,
		}

		r, err := repro.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		ctx := context.Background()
		result, err := r.Capture(ctx, repro.WithEvent(), repro.WithReason("test"))

		if err != nil {
			t.Errorf("Capture() error = %v", err)
		}
		if result == nil {
			t.Fatal("Capture() returned nil result")
		}
		if len(result.Events) != 1 {
			t.Errorf("result has %d events, want 1", len(result.Events))
		}
		if len(evtSys.emitted) != 1 {
			t.Errorf("event system emitted %d events, want 1", len(evtSys.emitted))
		}
		if evtSys.emitted[0].Type != event.TypeSnapshotCreated {
			t.Errorf("event Type = %v, want %v", evtSys.emitted[0].Type, event.TypeSnapshotCreated)
		}
		if evtSys.emitted[0].Source != event.SourceRepro {
			t.Errorf("event Source = %v, want %v", evtSys.emitted[0].Source, event.SourceRepro)
		}
	})

	t.Run("capture without event system but WithEvent specified", func(t *testing.T) {
		eng := &mockEngine{}
		cfg := &capture.Config{
			Engine: eng,
			// No EventSystem
		}

		r, err := repro.New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		ctx := context.Background()
		result, err := r.Capture(ctx, repro.WithEvent())

		if err != nil {
			t.Errorf("Capture() error = %v", err)
		}
		if result == nil {
			t.Fatal("Capture() returned nil result")
		}
		if len(result.Events) != 0 {
			t.Errorf("result has %d events, want 0 (no event system)", len(result.Events))
		}
	})
}

func TestRepro_Store(t *testing.T) {
	eng := &mockEngine{}
	cfg := &capture.Config{
		Engine: eng,
	}

	r, err := repro.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	snap := &snapshot.Snapshot{
		ID:     snapshot.NewID(),
		Source: snapshot.SourceRepro,
	}

	err = r.Store(ctx, snap)
	if err != nil {
		t.Errorf("Store() error = %v", err)
	}
	if len(eng.stored) != 1 {
		t.Errorf("engine stored %d snapshots, want 1", len(eng.stored))
	}
}

func TestRepro_CaptureAndStore(t *testing.T) {
	eng := &mockEngine{}
	cfg := &capture.Config{
		Engine: eng,
	}

	r, err := repro.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	result, err := r.CaptureAndStore(ctx, repro.WithReason("test"))

	if err != nil {
		t.Errorf("CaptureAndStore() error = %v", err)
	}
	if result == nil {
		t.Fatal("CaptureAndStore() returned nil result")
	}
	if result.Snapshot == nil {
		t.Error("CaptureAndStore() result has nil Snapshot")
	}
	if len(eng.captured) != 1 {
		t.Errorf("engine captured %d snapshots, want 1", len(eng.captured))
	}
	if len(eng.stored) != 1 {
		t.Errorf("engine stored %d snapshots, want 1", len(eng.stored))
	}
}

func TestRepro_CaptureWithCollectors(t *testing.T) {
	eng := &mockEngine{}
	cfg := &capture.Config{
		Engine: eng,
	}

	r, err := repro.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	result, err := r.Capture(ctx, repro.WithCollectors("os", "runtime"))

	if err != nil {
		t.Errorf("Capture() error = %v", err)
	}
	if result == nil {
		t.Fatal("Capture() returned nil result")
	}
	if result.Snapshot == nil {
		t.Error("Capture() result has nil Snapshot")
	}
}

func TestRepro_UsesSharedEngine(t *testing.T) {
	eng := &mockEngine{}
	cfg := &capture.Config{
		Engine: eng,
	}

	r, err := repro.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	// Capture multiple times
	_, err = r.Capture(ctx)
	if err != nil {
		t.Errorf("Capture() error = %v", err)
	}

	_, err = r.Capture(ctx)
	if err != nil {
		t.Errorf("Capture() error = %v", err)
	}

	// Verify both captures went through the same engine
	if len(eng.captured) != 2 {
		t.Errorf("engine captured %d snapshots, want 2", len(eng.captured))
	}

	// Verify all snapshots have the same source
	for i, snap := range eng.captured {
		if snap.Source != snapshot.SourceRepro {
			t.Errorf("snapshot[%d].Source = %v, want %v", i, snap.Source, snapshot.SourceRepro)
		}
	}
}
