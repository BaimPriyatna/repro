package errors_test

import (
	stderrors "errors"
	"strings"
	"testing"

	repro "github.com/BaimPriyatna/repro/src/core/errors"
)

func TestNew_ErrorString(t *testing.T) {
	err := repro.New(repro.CodeInvalidInput, "bad value")
	got := err.Error()
	if !strings.Contains(got, string(repro.CodeInvalidInput)) {
		t.Errorf("Error() = %q; want to contain %q", got, repro.CodeInvalidInput)
	}
	if !strings.Contains(got, "bad value") {
		t.Errorf("Error() = %q; want to contain %q", got, "bad value")
	}
}

func TestWrap_UnwrapCause(t *testing.T) {
	cause := stderrors.New("underlying failure")
	err := repro.Wrap(repro.CodeStorageFailure, "store error", cause)

	if !stderrors.Is(err, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
	if !strings.Contains(err.Error(), "underlying failure") {
		t.Errorf("Error() = %q; want cause message included", err.Error())
	}
}

func TestWithDetails_Chaining(t *testing.T) {
	err := repro.New(repro.CodeNotFound, "snapshot not found").
		WithDetails(map[string]any{"id": "snap-123"})
	if err.Details["id"] != "snap-123" {
		t.Errorf("Details[id] = %v; want snap-123", err.Details["id"])
	}
}

func TestWithRecovery_Chaining(t *testing.T) {
	err := repro.New(repro.CodeInternal, "unexpected state").
		WithRecovery("restart the daemon")
	if err.Recovery == "" {
		t.Error("Recovery should not be empty after WithRecovery")
	}
}

func TestNew_NilCause(t *testing.T) {
	err := repro.New(repro.CodeUnknown, "something")
	if err.Cause != nil {
		t.Error("New should not set a Cause")
	}
}

func TestIs(t *testing.T) {
	err := repro.New(repro.CodeInvalidInput, "bad param")
	if !repro.Is(err, repro.CodeInvalidInput) {
		t.Error("expected repro.Is to return true for matching code")
	}
	if repro.Is(err, repro.CodeNotFound) {
		t.Error("expected repro.Is to return false for different code")
	}
	if repro.Is(nil, repro.CodeInvalidInput) {
		t.Error("expected repro.Is to return false for nil error")
	}

	wrapped := repro.Wrap(repro.CodeStorageFailure, "storage error", err)
	if !repro.Is(wrapped, repro.CodeStorageFailure) {
		t.Error("expected repro.Is to match outermost code")
	}
	if !repro.Is(wrapped, repro.CodeInvalidInput) {
		t.Error("expected repro.Is to match unwrapped cause code")
	}
}
