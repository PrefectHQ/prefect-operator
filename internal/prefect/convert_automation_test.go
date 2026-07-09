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

package prefect

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

func ptr[T any](v T) *T { return &v }

var _ = Describe("ConvertToAutomationSpec", func() {
	newAutomation := func(trigger prefectiov1.PrefectAutomationTrigger) *prefectiov1.PrefectAutomation {
		return &prefectiov1.PrefectAutomation{
			ObjectMeta: metav1.ObjectMeta{Name: "auto", Namespace: "default"},
			Spec: prefectiov1.PrefectAutomationSpec{
				Name:    "my-automation",
				Trigger: trigger,
			},
		}
	}

	It("renders an event trigger with defaults and actions", func() {
		a := newAutomation(prefectiov1.PrefectAutomationTrigger{
			Event: &prefectiov1.PrefectEventTrigger{
				Posture:   "Reactive",
				Expect:    []string{"prefect.flow-run.Failed"},
				Threshold: ptr(1),
				Within:    ptr(60),
				Match:     &runtime.RawExtension{Raw: []byte(`{"prefect.resource.id":"prefect.flow-run.*"}`)},
			},
		})
		a.Spec.Actions = []prefectiov1.PrefectAutomationAction{
			{Type: "run-deployment", Source: ptr("selected"), DeploymentID: ptr("dep-1"),
				Parameters: &runtime.RawExtension{Raw: []byte(`{"x":1}`)}},
		}

		spec, err := ConvertToAutomationSpec(a, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(spec.Name).To(Equal("my-automation"))
		Expect(spec.Trigger["type"]).To(Equal("event"))
		Expect(spec.Trigger["posture"]).To(Equal("Reactive"))
		Expect(spec.Trigger["threshold"]).To(Equal(1))
		Expect(spec.Trigger["within"]).To(Equal(60))
		Expect(spec.Trigger["match"]).To(HaveKeyWithValue("prefect.resource.id", "prefect.flow-run.*"))
		// defaults present
		Expect(spec.Trigger["expect"]).To(Equal([]string{"prefect.flow-run.Failed"}))
		Expect(spec.Trigger["after"]).To(Equal([]string{}))
		Expect(spec.Trigger["match_related"]).To(Equal(map[string]any{}))

		Expect(spec.Actions).To(HaveLen(1))
		Expect(spec.Actions[0]).To(HaveKeyWithValue("type", "run-deployment"))
		Expect(spec.Actions[0]).To(HaveKeyWithValue("deployment_id", "dep-1"))
		Expect(spec.Actions[0]).To(HaveKeyWithValue("source", "selected"))
		Expect(spec.Actions[0]).To(HaveKeyWithValue("parameters", map[string]any{"x": float64(1)}))
	})

	It("renders a metric trigger with a fractional threshold", func() {
		a := newAutomation(prefectiov1.PrefectAutomationTrigger{
			Metric: &prefectiov1.PrefectMetricTrigger{
				Metric: prefectiov1.PrefectMetricQuery{
					Name:      "lateness",
					Operator:  ">=",
					Threshold: resource.MustParse("0.5"),
					Range:     3600,
					FiringFor: 600,
				},
			},
		})
		spec, err := ConvertToAutomationSpec(a, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(spec.Trigger["type"]).To(Equal("metric"))
		metric := spec.Trigger["metric"].(map[string]any)
		Expect(metric["name"]).To(Equal("lateness"))
		Expect(metric["operator"]).To(Equal(">="))
		Expect(metric["threshold"]).To(BeNumerically("~", 0.5, 0.0001))
		Expect(metric["range"]).To(Equal(3600))
		Expect(metric["firing_for"]).To(Equal(600))
		// actions must never be null for the API
		Expect(spec.Actions).To(Equal([]map[string]any{}))
	})

	It("renders a compound trigger with numeric require and nested children", func() {
		a := newAutomation(prefectiov1.PrefectAutomationTrigger{
			Compound: &prefectiov1.PrefectCompoundTrigger{
				Require: "2",
				Within:  ptr(120),
				Triggers: []prefectiov1.PrefectChildTrigger{
					{Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"}},
					{Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"}},
				},
			},
		})
		spec, err := ConvertToAutomationSpec(a, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(spec.Trigger["type"]).To(Equal("compound"))
		Expect(spec.Trigger["require"]).To(Equal(2))
		Expect(spec.Trigger["within"]).To(Equal(120))
		Expect(spec.Trigger["triggers"]).To(HaveLen(2))
	})

	It("keeps string require values (any/all)", func() {
		a := newAutomation(prefectiov1.PrefectAutomationTrigger{
			Sequence: &prefectiov1.PrefectSequenceTrigger{
				Triggers: []prefectiov1.PrefectChildTrigger{
					{Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"}},
				},
			},
		})
		spec, err := ConvertToAutomationSpec(a, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(spec.Trigger["type"]).To(Equal("sequence"))
		Expect(spec.Trigger["triggers"]).To(HaveLen(1))
	})

	It("errors when no trigger member is set", func() {
		a := newAutomation(prefectiov1.PrefectAutomationTrigger{})
		_, err := ConvertToAutomationSpec(a, nil)
		Expect(err).To(HaveOccurred())
	})

	It("resolves a run-deployment action's deploymentName to an ID", func() {
		a := newAutomation(prefectiov1.PrefectAutomationTrigger{
			Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"},
		})
		a.Spec.Actions = []prefectiov1.PrefectAutomationAction{
			{Type: "run-deployment", Source: ptr("selected"), DeploymentName: ptr("adsb-airnav-jam")},
		}
		Expect(DeploymentNamesReferenced(a)).To(Equal([]string{"adsb-airnav-jam"}))

		spec, err := ConvertToAutomationSpec(a, map[string]string{"adsb-airnav-jam": "uuid-123"})
		Expect(err).NotTo(HaveOccurred())
		Expect(spec.Actions[0]).To(HaveKeyWithValue("deployment_id", "uuid-123"))
		Expect(spec.Actions[0]).NotTo(HaveKey("deploymentName"))
	})

	It("errors when a deploymentName can't be resolved", func() {
		a := newAutomation(prefectiov1.PrefectAutomationTrigger{
			Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"},
		})
		a.Spec.Actions = []prefectiov1.PrefectAutomationAction{
			{Type: "run-deployment", DeploymentName: ptr("missing")},
		}
		_, err := ConvertToAutomationSpec(a, map[string]string{})
		Expect(err).To(HaveOccurred())
	})

	It("propagates invalid JSON in action parameters", func() {
		a := newAutomation(prefectiov1.PrefectAutomationTrigger{
			Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"},
		})
		a.Spec.Actions = []prefectiov1.PrefectAutomationAction{
			{
				Type:       "run-deployment",
				Source:     ptr("inferred"),
				Parameters: &runtime.RawExtension{Raw: []byte(`{"bad": json}`)},
			},
		}
		_, err := ConvertToAutomationSpec(a, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("parameters"))
	})

	It("propagates invalid JSON in action jobVariables", func() {
		a := newAutomation(prefectiov1.PrefectAutomationTrigger{
			Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"},
		})
		a.Spec.Actions = []prefectiov1.PrefectAutomationAction{
			{
				Type:         "run-deployment",
				Source:       ptr("inferred"),
				JobVariables: &runtime.RawExtension{Raw: []byte(`{"bad": json}`)},
			},
		}
		_, err := ConvertToAutomationSpec(a, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("job_variables"))
	})
})
