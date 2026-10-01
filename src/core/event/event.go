// Package event defines Event types and the Event System.
//
// Events supply temporal context that snapshots alone cannot. RelatedSnapshotID
// is an optional reference, not a parent/child link.
package event

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
)

// ID is a stable, unique event identifier.
type ID string

// NewID generates a cryptographically secure random UUID v4 identifier.
func NewID() ID {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return ID(fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
}

// Type classifies the kind of occurrence an Event records.
type Type string

const (
	// TypeFileChanged records a filesystem modification.
	TypeFileChanged Type = "FILE_CHANGED"

	// TypePackageInstalled records the addition of a package.
	TypePackageInstalled Type = "PACKAGE_INSTALLED"

	// TypePackageRemoved records the removal of a package.
	TypePackageRemoved Type = "PACKAGE_REMOVED"

	// TypeConfigChanged records a configuration-file modification.
	TypeConfigChanged Type = "CONFIG_CHANGED"

	// TypeGitCommit records a git commit.
	TypeGitCommit Type = "GIT_COMMIT"

	// TypeCommandExecuted records that a command was run by the user.
	TypeCommandExecuted Type = "COMMAND_EXECUTED"

	// TypeRuntimeChanged records a change in the active runtime version.
	TypeRuntimeChanged Type = "RUNTIME_CHANGED"

	// TypeSnapshotCreated records the creation of a new snapshot.
	TypeSnapshotCreated Type = "SNAPSHOT_CREATED"

	// TypeSnapshotDeleted records the deletion of a snapshot.
	TypeSnapshotDeleted Type = "SNAPSHOT_DELETED"
)

// Source describes the subsystem that emitted the event.
type Source string

const (
	// SourceWatchdog means the event originated from the Watchdog sensor.
	SourceWatchdog Source = "watchdog"

	// SourceRepro means the event originated from the Repro capture module.
	SourceRepro Source = "repro"

	// SourceTimeCapsule means the event originated from the TimeCapsule module.
	SourceTimeCapsule Source = "timecapsule"

	// SourceUser means the event was created directly by a user action.
	SourceUser Source = "user"
)

// Event is a lightweight record of an occurrence in the development environment.
// Events provide temporal context around snapshots.
//
// Events are not subordinate to snapshots. RelatedSnapshotID is an optional
// back-reference only — an Event may exist with no snapshot association at all
// (e.g. a COMMAND_EXECUTED event between two snapshots).
type Event struct {
	// ID is the unique identifier for this event.
	ID ID `json:"id"`

	// Timestamp is the wall-clock time when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Type classifies what kind of occurrence this event records.
	Type Type `json:"type"`

	// Source identifies the subsystem that emitted this event.
	Source Source `json:"source"`

	// Subject is a human-readable identifier for the primary resource involved
	// (e.g. a file path, package name, or command string).
	Subject string `json:"subject"`

	// Metadata holds optional structured context specific to the event type.
	// Keys and value shapes are event-type-specific. The storage layer must
	// preserve unknown keys without modification.
	Metadata map[string]any `json:"metadata,omitempty"`

	// RelatedSnapshotID is an *optional* reference to a snapshot that was
	// current at the time of the event. It is NOT a parent/child relationship
	// . An empty value means no association.
	RelatedSnapshotID snapshot.ID `json:"related_snapshot_id,omitempty"`
}

// Option configures an Event during construction.
type Option func(*Event)

// WithID sets an explicit ID on the event instead of generating a random UUID.
func WithID(id ID) Option {
	return func(e *Event) {
		e.ID = id
	}
}

// WithTimestamp sets an explicit timestamp on the event instead of time.Now.UTC.
func WithTimestamp(t time.Time) Option {
	return func(e *Event) {
		e.Timestamp = t.UTC()
	}
}

// WithMetadata sets metadata on the event. It makes a defensive shallow/map copy
// so external mutations to the map do not affect the event.
func WithMetadata(meta map[string]any) Option {
	return func(e *Event) {
		if meta == nil {
			e.Metadata = nil
			return
		}
		copied := make(map[string]any, len(meta))
		for k, v := range meta {
			copied[k] = v
		}
		e.Metadata = copied
	}
}

// WithSnapshot associates the event with an optional related snapshot ID.
func WithSnapshot(snapID snapshot.ID) Option {
	return func(e *Event) {
		e.RelatedSnapshotID = snapID
	}
}

// New creates and validates a new Event with sensible defaults (UUID v4, UTC timestamp).
func New(eventType Type, source Source, subject string, opts ...Option) (*Event, error) {
	evt := &Event{
		ID:        NewID(),
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Source:    source,
		Subject:   subject,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(evt)
		}
	}

	if err := evt.Validate(); err != nil {
		return nil, err
	}

	return evt, nil
}

// Validate checks whether an Event satisfies core domain invariants.
func (e *Event) Validate() error {
	if e == nil {
		return errors.New(errors.CodeInvalidInput, "event is nil")
	}
	if e.ID == "" {
		return errors.New(errors.CodeInvalidInput, "event ID cannot be empty")
	}
	if e.Timestamp.IsZero() {
		return errors.New(errors.CodeInvalidInput, "event timestamp cannot be zero")
	}
	if e.Type == "" {
		return errors.New(errors.CodeInvalidInput, "event type cannot be empty")
	}
	if e.Source == "" {
		return errors.New(errors.CodeInvalidInput, "event source cannot be empty")
	}
	if e.Subject == "" {
		return errors.New(errors.CodeInvalidInput, "event subject cannot be empty")
	}
	return nil
}

// HasSnapshot reports whether this event is associated with a snapshot.
func (e *Event) HasSnapshot() bool {
	return e.RelatedSnapshotID != ""
}

// Clone returns an independent deep copy of the event.
func (e *Event) Clone() *Event {
	if e == nil {
		return nil
	}
	c := *e
	if e.Metadata != nil {
		c.Metadata = make(map[string]any, len(e.Metadata))
		for k, v := range e.Metadata {
			c.Metadata[k] = v
		}
	}
	return &c
}
