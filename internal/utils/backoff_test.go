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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

var _ = Describe("Backoff utilities", func() {
	Describe("CalculateBackoffDelay", func() {
		It("should return progressive delays", func() {
			testCases := []struct {
				attempts int
				expected time.Duration
			}{
				{0, 15 * time.Second},
				{1, 30 * time.Second},
				{2, 60 * time.Second},
				{3, 120 * time.Second},
				{4, 120 * time.Second},  // Max delay
				{10, 120 * time.Second}, // Still max delay
			}

			for _, tc := range testCases {
				delay := CalculateBackoffDelay(tc.attempts)
				Expect(delay).To(Equal(tc.expected), "Attempt %d should have delay %v", tc.attempts, tc.expected)
			}
		})
	})

	Describe("Retry count management", func() {
		var workPool *prefectiov1.PrefectWorkPool

		BeforeEach(func() {
			workPool = &prefectiov1.PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
			}
		})

		It("should start with zero retry count", func() {
			count := GetRetryCount(workPool)
			Expect(count).To(Equal(0))
		})

		It("should increment retry count", func() {
			IncrementRetryCount(workPool)
			Expect(GetRetryCount(workPool)).To(Equal(1))

			IncrementRetryCount(workPool)
			Expect(GetRetryCount(workPool)).To(Equal(2))
		})

		It("should reset retry count", func() {
			IncrementRetryCount(workPool)
			IncrementRetryCount(workPool)
			Expect(GetRetryCount(workPool)).To(Equal(2))

			ResetRetryCount(workPool)
			Expect(GetRetryCount(workPool)).To(Equal(0))
		})

		It("should detect when to stop retrying", func() {
			for i := 0; i < MaxRetryAttempts-1; i++ {
				IncrementRetryCount(workPool)
				Expect(ShouldStopRetrying(workPool)).To(BeFalse())
			}

			IncrementRetryCount(workPool)
			Expect(ShouldStopRetrying(workPool)).To(BeTrue())
		})

		It("should handle missing annotations gracefully", func() {
			workPool.SetAnnotations(nil)
			count := GetRetryCount(workPool)
			Expect(count).To(Equal(0))

			IncrementRetryCount(workPool)
			Expect(GetRetryCount(workPool)).To(Equal(1))
		})

		It("should handle invalid annotation values", func() {
			workPool.SetAnnotations(map[string]string{
				RetryCountAnnotation: "invalid",
			})
			count := GetRetryCount(workPool)
			Expect(count).To(Equal(0))
		})
	})
})
