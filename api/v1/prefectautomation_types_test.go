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

package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PrefectAutomation Validate", func() {
	// eventTrigger is a minimal valid trigger so tests can focus on action rules.
	eventTrigger := PrefectAutomationTrigger{
		Event: &PrefectEventTrigger{Posture: "Reactive"},
	}

	automationWithActions := func(actions ...PrefectAutomationAction) *PrefectAutomation {
		return &PrefectAutomation{
			Spec: PrefectAutomationSpec{
				Name:    "a",
				Trigger: eventTrigger,
				Actions: actions,
			},
		}
	}

	Describe("trigger union", func() {
		It("accepts exactly one trigger member", func() {
			a := &PrefectAutomation{Spec: PrefectAutomationSpec{Name: "a", Trigger: eventTrigger}}
			Expect(a.Validate()).To(Succeed())
		})

		It("rejects zero trigger members", func() {
			a := &PrefectAutomation{Spec: PrefectAutomationSpec{Name: "a"}}
			Expect(a.Validate()).To(HaveOccurred())
		})

		It("rejects more than one trigger member", func() {
			a := &PrefectAutomation{Spec: PrefectAutomationSpec{
				Name: "a",
				Trigger: PrefectAutomationTrigger{
					Event:  &PrefectEventTrigger{Posture: "Reactive"},
					Metric: &PrefectMetricTrigger{},
				},
			}}
			Expect(a.Validate()).To(HaveOccurred())
		})
	})

	Describe("deployment target actions", func() {
		It("rejects setting both deploymentId and deploymentName", func() {
			a := automationWithActions(PrefectAutomationAction{
				Type:           "run-deployment",
				Source:         new("selected"),
				DeploymentID:   new("dep-id"),
				DeploymentName: new("dep-name"),
			})
			err := a.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("mutually exclusive"))
		})

		It("rejects both even for non-deployment action types", func() {
			a := automationWithActions(PrefectAutomationAction{
				Type:           "pause-automation",
				DeploymentID:   new("dep-id"),
				DeploymentName: new("dep-name"),
			})
			Expect(a.Validate()).To(HaveOccurred())
		})

		It("requires a target when source is selected", func() {
			a := automationWithActions(PrefectAutomationAction{
				Type:   "run-deployment",
				Source: new("selected"),
			})
			err := a.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("selected"))
		})

		It("requires a target when source is unset (defaults to selected)", func() {
			a := automationWithActions(PrefectAutomationAction{
				Type: "run-deployment",
			})
			Expect(a.Validate()).To(HaveOccurred())
		})

		It("accepts a selected action with deploymentId only", func() {
			a := automationWithActions(PrefectAutomationAction{
				Type:         "run-deployment",
				Source:       new("selected"),
				DeploymentID: new("dep-id"),
			})
			Expect(a.Validate()).To(Succeed())
		})

		It("accepts a selected action with deploymentName only", func() {
			a := automationWithActions(PrefectAutomationAction{
				Type:           "pause-deployment",
				Source:         new("selected"),
				DeploymentName: new("dep-name"),
			})
			Expect(a.Validate()).To(Succeed())
		})

		It("rejects a target when source is inferred", func() {
			a := automationWithActions(PrefectAutomationAction{
				Type:         "run-deployment",
				Source:       new("inferred"),
				DeploymentID: new("dep-id"),
			})
			err := a.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("inferred"))
		})

		It("accepts an inferred action with no target", func() {
			a := automationWithActions(PrefectAutomationAction{
				Type:   "resume-deployment",
				Source: new("inferred"),
			})
			Expect(a.Validate()).To(Succeed())
		})

		It("validates actions across all three lists", func() {
			a := &PrefectAutomation{Spec: PrefectAutomationSpec{
				Name:    "a",
				Trigger: eventTrigger,
				ActionsOnResolve: []PrefectAutomationAction{
					{Type: "run-deployment", Source: new("selected")},
				},
			}}
			err := a.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("actionsOnResolve"))
		})
	})

	Describe("non-deployment actions", func() {
		It("does not require a deployment target", func() {
			a := automationWithActions(PrefectAutomationAction{
				Type:    "send-notification",
				Subject: new("hi"),
			})
			Expect(a.Validate()).To(Succeed())
		})
	})
})
