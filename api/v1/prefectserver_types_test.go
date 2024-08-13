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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PrefectServer type", func() {
	It("can be deep copied", func() {
		original := &PrefectServer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: PrefectServerSpec{
				Version: ptr.To("0.0.1"),
				Image:   ptr.To("prefecthq/prefect:0.0.1"),
				SQLite: &SQLiteConfiguration{
					StorageClassName: "standard",
					Size:             resource.MustParse("1Gi"),
				},
			},
		}

		copied := original.DeepCopy()

		Expect(copied).To(Equal(original))
		Expect(copied).NotTo(BeIdenticalTo(original))
	})
})
