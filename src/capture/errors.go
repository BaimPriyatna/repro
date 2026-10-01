package capture

import "github.com/BaimPriyatna/repro/src/core/errors"

// Capture module error codes and messages.
var (
	// ErrEngineRequired is returned when Config.Engine is nil.
	ErrEngineRequired = errors.New(errors.CodeInvalidInput, "snapshot engine is required")

	// ErrEventSystemRequired is returned when EventSystem is required but nil.
	ErrEventSystemRequired = errors.New(errors.CodeInvalidInput, "event system is required")

	// ErrInvalidInterval is returned for invalid time intervals.
	ErrInvalidInterval = errors.New(errors.CodeInvalidInput, "invalid time interval")

	// ErrAlreadyRunning is returned when attempting to start an already-running module.
	ErrAlreadyRunning = errors.New(errors.CodeInvalidInput, "module is already running")

	// ErrNotRunning is returned when attempting to stop a non-running module.
	ErrNotRunning = errors.New(errors.CodeInvalidInput, "module is not running")
)
