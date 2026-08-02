package scene

import (
	"time"

	"github.com/colinleefish/rmb-desktop/internal/model"
)

func shouldRunL2(
	now time.Time,
	l1Status string,
	l2Status string,
	l1AdvancedAt *int64,
	delayAfterL1 time.Duration,
) bool {
	if l1Status == model.PipelineStatusRunning {
		return false
	}
	if l2Status != model.PipelineStatusPending && l2Status != model.PipelineStatusFailed {
		return false
	}
	if l1AdvancedAt == nil {
		return false
	}
	if delayAfterL1 > 0 {
		advanced := time.UnixMilli(*l1AdvancedAt).UTC()
		if now.Sub(advanced) < delayAfterL1 {
			return false
		}
	}
	return true
}
