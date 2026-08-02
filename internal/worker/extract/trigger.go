package extract

import (
	"time"

	"github.com/colinleefish/rmb-desktop/internal/model"
)

func shouldRunL1(
	now time.Time,
	l1Status string,
	unprocessedTurns int,
	turnsSinceAdvanced int,
	warmupThreshold int,
	everyN int,
	warmupEnabled bool,
	idleSeconds time.Duration,
	lastTurnAt time.Time,
) bool {
	if unprocessedTurns <= 0 {
		return false
	}
	if l1Status == model.PipelineStatusPending || l1Status == model.PipelineStatusFailed {
		return true
	}

	threshold := everyN
	if everyN <= 0 {
		threshold = 8
	}
	if warmupEnabled && warmupThreshold > 0 && warmupThreshold < threshold {
		threshold = warmupThreshold
	}
	if turnsSinceAdvanced >= threshold {
		return true
	}
	if idleSeconds > 0 && !lastTurnAt.IsZero() && now.Sub(lastTurnAt) >= idleSeconds {
		return true
	}
	return false
}

func nextWarmupThreshold(current, everyN int, warmupEnabled bool) int {
	if !warmupEnabled || everyN <= 0 {
		return everyN
	}
	if current <= 0 {
		return 2
	}
	next := current * 2
	if next > everyN {
		return everyN
	}
	return next
}
