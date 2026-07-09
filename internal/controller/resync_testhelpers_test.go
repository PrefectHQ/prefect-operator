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

package controller

import (
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

// testResyncInterval is the resync interval the controller tests configure so
// requeue assertions are deterministic (above the MinResyncInterval floor).
const testResyncInterval = 30 * time.Second

// BeJitteredResync asserts a RequeueAfter falls within the jitter window that
// JitterResyncInterval produces for the given base interval: [base, base+10%].
func BeJitteredResync(base time.Duration) types.GomegaMatcher {
	return And(
		BeNumerically(">=", base),
		BeNumerically("<=", base+base/10+1),
	)
}
