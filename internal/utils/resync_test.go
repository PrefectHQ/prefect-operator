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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResyncInterval(t *testing.T) {
	def := 5 * time.Minute

	tests := []struct {
		name         string
		specInterval *metav1.Duration
		want         time.Duration
	}{
		{"nil spec uses default", nil, def},
		{"zero spec uses default", &metav1.Duration{Duration: 0}, def},
		{"spec overrides default", &metav1.Duration{Duration: 90 * time.Second}, 90 * time.Second},
		{"spec below floor clamps", &metav1.Duration{Duration: time.Second}, MinResyncInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResyncInterval(tt.specInterval, def); got != tt.want {
				t.Fatalf("ResyncInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResyncIntervalClampsDefaultBelowFloor(t *testing.T) {
	// A default below the floor (e.g. operator misconfigured to 0) still clamps.
	if got := ResyncInterval(nil, 0); got != MinResyncInterval {
		t.Fatalf("ResyncInterval(nil, 0) = %v, want %v", got, MinResyncInterval)
	}
}

func TestJitterResyncInterval(t *testing.T) {
	base := 30 * time.Second
	// Sample repeatedly: every result must stay within [base, base+10%].
	for i := 0; i < 1000; i++ {
		got := JitterResyncInterval(base)
		if got < base || got > base+base/10+1 {
			t.Fatalf("JitterResyncInterval(%v) = %v, out of [%v, %v]", base, got, base, base+base/10+1)
		}
	}
}

func TestJitterResyncIntervalZero(t *testing.T) {
	if got := JitterResyncInterval(0); got != 0 {
		t.Fatalf("JitterResyncInterval(0) = %v, want 0", got)
	}
}
