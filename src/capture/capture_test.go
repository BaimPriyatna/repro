package capture_test

import (
	"context"
	"testing"

	"github.com/BaimPriyatna/repro/src/capture"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/engine"
)

// mockEngine implements capture.Engine for testing.
type mockEngine struct {
	captureFunc func(ctx context.Context, source snapshot.Source, opts ...engine.CaptureOption) (*snapshot.Snapshot, error)
	storeFunc   func(ctx context.Context, snap *snapshot.Snapshot) error
	loadFunc    func(ctx context.Context, id snapshot.ID) (*snapshot.Snapshot, error)
}

func (m *mockEngine) Capture(ctx context.Context, source snapshot.Source, opts ...engine.CaptureOption) (*snapshot.Snapshot, error) {
	if m.captureFunc != nil {
		return m.captureFunc(ctx, source, opts...)
	}
	return &snapshot.Snapshot{ID: snapshot.NewID(), Source: source}, nil
}

func (m *mockEngine) Store(ctx context.Context, snap *snapshot.Snapshot) error {
	if m.storeFunc != nil {
		return m.storeFunc(ctx, snap)
	}
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
}

func (m *mockEventSystem) Emit(ctx context.Context, eventType event.Type, source event.Source, subject string, opts ...event.Option) (*event.Event, error) {
	if m.emitFunc != nil {
		return m.emitFunc(ctx, eventType, source, subject, opts...)
	}
	evt, _ := event.New(eventType, source, subject, opts...)
	return evt, nil
}

func (m *mockEventSystem) Record(ctx context.Context, evt *event.Event) error {
	if m.recordFunc != nil {
		return m.recordFunc(ctx, evt)
	}
	return nil
}

func TestConfig_Validate(t *testing.T) {
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
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_CaptureOptions(t *testing.T) {
	t.Run("no options", func(t *testing.T) {
		cfg := &capture.Config{
			Engine: &mockEngine{},
		}
		opts := cfg.CaptureOptions("")
		if len(opts) != 0 {
			t.Errorf("expected 0 options, got %d", len(opts))
		}
	})

	t.Run("with parent ID", func(t *testing.T) {
		cfg := &capture.Config{
			Engine: &mockEngine{},
		}
		parentID := snapshot.NewID()
		opts := cfg.CaptureOptions(parentID)
		if len(opts) != 1 {
			t.Errorf("expected 1 option, got %d", len(opts))
		}
	})

	t.Run("with labels", func(t *testing.T) {
		cfg := &capture.Config{
			Engine: &mockEngine{},
			Labels: map[string]string{
				"env":  "dev",
				"user": "test",
			},
		}
		opts := cfg.CaptureOptions("")
		if len(opts) != 1 {
			t.Errorf("expected 1 option, got %d", len(opts))
		}
	})

	t.Run("with collector names", func(t *testing.T) {
		cfg := &capture.Config{
			Engine:         &mockEngine{},
			CollectorNames: []string{"os", "runtime"},
		}
		opts := cfg.CaptureOptions("")
		if len(opts) != 1 {
			t.Errorf("expected 1 option, got %d", len(opts))
		}
	})

	t.Run("with all options", func(t *testing.T) {
		cfg := &capture.Config{
			Engine: &mockEngine{},
			Labels: map[string]string{
				"env": "dev",
			},
			CollectorNames: []string{"os"},
		}
		parentID := snapshot.NewID()
		opts := cfg.CaptureOptions(parentID)
		if len(opts) != 3 {
			t.Errorf("expected 3 options, got %d", len(opts))
		}
	})
}

func TestResult(t *testing.T) {
	t.Run("successful result", func(t *testing.T) {
		snap := &snapshot.Snapshot{ID: snapshot.NewID()}
		evt, _ := event.New(event.TypeSnapshotCreated, event.SourceRepro, "test")

		result := &capture.Result{
			Snapshot: snap,
			Events:   []*event.Event{evt},
			Error:    nil,
		}

		if result.Snapshot == nil {
			t.Error("expected snapshot to be set")
		}
		if len(result.Events) != 1 {
			t.Errorf("expected 1 event, got %d", len(result.Events))
		}
		if result.Error != nil {
			t.Errorf("expected no error, got %v", result.Error)
		}
	})

	t.Run("error result", func(t *testing.T) {
		result := &capture.Result{
			Error: capture.ErrEngineRequired,
		}

		if result.Snapshot != nil {
			t.Error("expected no snapshot")
		}
		if result.Error == nil {
			t.Error("expected error to be set")
		}
	})
}
