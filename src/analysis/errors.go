package analysis

import "github.com/BaimPriyatna/repro/src/core/errors"

// Analysis module error definitions.
var (
	// ErrSnapshotNotFound is returned when a required snapshot does not exist.
	ErrSnapshotNotFound = errors.New(errors.CodeNotFound, "snapshot not found")

	// ErrInvalidInput is returned for invalid analysis parameters.
	ErrInvalidInput = errors.New(errors.CodeInvalidInput, "invalid analysis input")

	// ErrInsufficientData is returned when analysis cannot proceed due to missing data.
	ErrInsufficientData = errors.New(errors.CodeInvalidInput, "insufficient data for analysis")

	// ErrNoBaseline is returned when a baseline snapshot is required but not provided.
	ErrNoBaseline = errors.New(errors.CodeInvalidInput, "no baseline snapshot specified")

	// ErrEmptyHistory is returned when history traversal yields no snapshots.
	ErrEmptyHistory = errors.New(errors.CodeNotFound, "empty history chain")
)
