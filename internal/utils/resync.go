/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MinResyncInterval is the floor for the periodic drift-detection resync. It
// keeps a per-resource spec.interval from imposing an abusive load on the
// Prefect API.
const MinResyncInterval = 10 * time.Second

// ResyncInterval returns the effective drift-detection interval for a resource:
// its per-resource spec.interval when set, otherwise the operator-wide default.
// The result is clamped to MinResyncInterval.
func ResyncInterval(specInterval *metav1.Duration, defaultInterval time.Duration) time.Duration {
	interval := defaultInterval
	if specInterval != nil {
		interval = specInterval.Duration
	}
	if interval < MinResyncInterval {
		interval = MinResyncInterval
	}
	return interval
}

// NextResyncDelay returns the remaining time until the next jittered resync.
// Measuring from the last successful sync prevents reconciles caused by restarts
// or watch events from postponing drift detection by another full interval.
func NextResyncDelay(lastSyncTime *metav1.Time, interval time.Duration) time.Duration {
	jitteredInterval := JitterResyncInterval(interval)
	if lastSyncTime == nil {
		return jitteredInterval
	}

	elapsed := time.Since(lastSyncTime.Time)
	if elapsed <= 0 {
		return jitteredInterval
	}
	if elapsed >= jitteredInterval {
		return 0
	}
	return jitteredInterval - elapsed
}

// JitterResyncInterval applies up to ~10% positive jitter to an interval so that
// many resources sharing one interval don't realign into a thundering herd
// against the Prefect API.
func JitterResyncInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return interval
	}
	// #nosec G404 -- jitter is for load spreading, not security.
	return interval + time.Duration(rand.Int63n(int64(interval)/10+1))
}
