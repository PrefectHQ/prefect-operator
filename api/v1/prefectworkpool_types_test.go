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
	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PrefectWorkPool type", func() {
	It("can be deep copied", func() {
		original := &PrefectWorkPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: PrefectWorkPoolSpec{
				Version: ptr.To("0.0.1"),
				Image:   ptr.To("prefecthq/prefect:0.0.1"),
				Server: PrefectServerReference{
					Namespace: "default",
					Name:      "prefect",
				},
				Type:    "kubernetes",
				Workers: int32(2),
			}}

		copied := original.DeepCopy()

		Expect(copied).To(Equal(original))
		Expect(copied).NotTo(BeIdenticalTo(original))
	})

	Describe("EntrypointArguments", func() {
		It("should use exact work pool name for names starting with 'prefect'", func() {
			workPool := &PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prefect-work-pool",
				},
				Spec: PrefectWorkPoolSpec{
					Type: "kubernetes",
				},
			}

			args := workPool.EntrypointArguments()
			Expect(args).To(ContainElement("prefect-work-pool"))
			Expect(args).To(Equal([]string{
				"prefect", "worker", "start",
				"--pool", "prefect-work-pool", "--type", "kubernetes",
				"--with-healthcheck",
			}))
		})

		It("should use exact work pool name for names not starting with 'prefect'", func() {
			workPool := &PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-work-pool",
				},
				Spec: PrefectWorkPoolSpec{
					Type: "process",
				},
			}

			args := workPool.EntrypointArguments()
			Expect(args).To(ContainElement("my-work-pool"))
			Expect(args).To(Equal([]string{
				"prefect", "worker", "start",
				"--pool", "my-work-pool", "--type", "process",
				"--with-healthcheck",
			}))
		})

		It("ensures naming consistency between controller and worker", func() {
			workPool := &PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prefect-production",
				},
				Spec: PrefectWorkPoolSpec{
					Type: "kubernetes",
				},
			}

			controllerName := workPool.Name

			workerArgs := workPool.EntrypointArguments()
			var workerPoolName string
			for i, arg := range workerArgs {
				if arg == "--pool" && i+1 < len(workerArgs) {
					workerPoolName = workerArgs[i+1]
					break
				}
			}

			Expect(controllerName).To(Equal("prefect-production"))
			Expect(workerPoolName).To(Equal("prefect-production"))
			Expect(controllerName).To(Equal(workerPoolName))
		})

		It("should work consistently for all naming patterns", func() {
			testCases := []struct {
				name         string
				expectedPool string
			}{
				{"prefect", "prefect"},
				{"prefect-k8s", "prefect-k8s"},
				{"prefect123", "prefect123"},
				{"k8s-pool", "k8s-pool"},
				{"my-prefect-pool", "my-prefect-pool"},
				{"PREFECT-POOL", "PREFECT-POOL"},
				{"production-pool", "production-pool"},
				{"dev", "dev"},
			}

			for _, tc := range testCases {
				workPool := &PrefectWorkPool{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.name,
					},
					Spec: PrefectWorkPoolSpec{
						Type: "process",
					},
				}

				args := workPool.EntrypointArguments()
				Expect(args).To(ContainElement(tc.expectedPool),
					"Work pool %s should use pool name %s", tc.name, tc.expectedPool)
			}
		})
	})
})
