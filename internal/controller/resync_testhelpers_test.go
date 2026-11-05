package controller

import (
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

// testResyncInterval is the resync interval the controller tests configure so
// requeue assertions are deterministic (above the MinResyncInterval floor).
const testResyncInterval = 30 * time.Second

// BeJitteredResync asserts a RequeueAfter falls within the jitter window for a
// resource synced within the last second. The lower tolerance accounts for
// NextResyncDelay subtracting time elapsed since LastSyncTime was stamped.
func BeJitteredResync(base time.Duration) types.GomegaMatcher {
	return And(
		BeNumerically(">=", base-time.Second),
		BeNumerically("<=", base+base/10+1),
	)
}
