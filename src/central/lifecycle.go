package central

import (
	"time"
)

// DefaultThresholds returns standard lifecycle transition day thresholds.
func DefaultThresholds() LifecycleThresholds {
	return LifecycleThresholds{
		ActiveDays:  7,
		IdleDays:    30,
		DormantDays: 90,
	}
}

// ComputeStatus calculates the project lifecycle status based on elapsed time and thresholds.
func ComputeStatus(now time.Time, lastActivity time.Time, thresholds LifecycleThresholds, archived bool) LifecycleStatus {
	if archived {
		return StatusArchived
	}

	if thresholds.ActiveDays <= 0 {
		thresholds.ActiveDays = 7
	}
	if thresholds.IdleDays <= thresholds.ActiveDays {
		thresholds.IdleDays = 30
	}
	if thresholds.DormantDays <= thresholds.IdleDays {
		thresholds.DormantDays = 90
	}

	if lastActivity.IsZero() {
		return StatusAbandoned
	}

	diff := now.Sub(lastActivity)
	days := int(diff.Hours() / 24)
	if days < 0 {
		days = 0
	}

	switch {
	case days <= thresholds.ActiveDays:
		return StatusActive
	case days <= thresholds.IdleDays:
		return StatusIdle
	case days <= thresholds.DormantDays:
		return StatusDormant
	default:
		return StatusAbandoned
	}
}
