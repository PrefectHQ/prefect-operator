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
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	RetryCountAnnotation = "prefect.io/retry-count"
	MaxRetryAttempts     = 10
)

// CalculateBackoffDelay returns a progressive delay based on retry attempts
// Uses exponential backoff: 15s, 30s, 60s, 120s (max)
func CalculateBackoffDelay(attempts int) time.Duration {
	delays := []time.Duration{
		15 * time.Second,
		30 * time.Second,
		60 * time.Second,
		120 * time.Second,
	}

	if attempts >= len(delays) {
		return delays[len(delays)-1] // Max delay
	}

	return delays[attempts]
}

// GetRetryCount retrieves the current retry count from object annotations
func GetRetryCount(obj client.Object) int {
	if obj.GetAnnotations() == nil {
		return 0
	}

	countStr, exists := obj.GetAnnotations()[RetryCountAnnotation]
	if !exists {
		return 0
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return 0
	}

	return count
}

// IncrementRetryCount increments the retry count annotation on an object
func IncrementRetryCount(obj client.Object) {
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(make(map[string]string))
	}

	currentCount := GetRetryCount(obj)
	newCount := currentCount + 1

	annotations := obj.GetAnnotations()
	annotations[RetryCountAnnotation] = strconv.Itoa(newCount)
	obj.SetAnnotations(annotations)
}

// ResetRetryCount clears the retry count annotation on successful operations
func ResetRetryCount(obj client.Object) {
	if obj.GetAnnotations() == nil {
		return
	}

	annotations := obj.GetAnnotations()
	delete(annotations, RetryCountAnnotation)
	obj.SetAnnotations(annotations)
}

// ShouldStopRetrying determines if we've exceeded max retry attempts
func ShouldStopRetrying(obj client.Object) bool {
	return GetRetryCount(obj) >= MaxRetryAttempts
}
