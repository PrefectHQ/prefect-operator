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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

var _ = Describe("Server health utilities", func() {
	Describe("IsInClusterServer", func() {
		It("should identify in-cluster servers", func() {
			serverRef := &prefectiov1.PrefectServerReference{
				Name:      "prefect-ephemeral",
				Namespace: "default",
			}
			Expect(IsInClusterServer(serverRef)).To(BeTrue())
		})

		It("should identify external servers with RemoteAPIURL", func() {
			serverRef := &prefectiov1.PrefectServerReference{
				Name:         "prefect-ephemeral",
				Namespace:    "default",
				RemoteAPIURL: ptr.To("https://api.prefect.cloud/api/accounts/123/workspaces/456"),
			}
			Expect(IsInClusterServer(serverRef)).To(BeFalse())
		})

		It("should identify Prefect Cloud servers", func() {
			serverRef := &prefectiov1.PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.cloud/api/accounts/123/workspaces/456"),
				AccountID:    ptr.To("123"),
				WorkspaceID:  ptr.To("456"),
			}
			Expect(IsInClusterServer(serverRef)).To(BeFalse())
		})

		It("should handle empty server reference", func() {
			serverRef := &prefectiov1.PrefectServerReference{}
			Expect(IsInClusterServer(serverRef)).To(BeFalse())
		})
	})

	Describe("CheckPrefectServerHealth", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		It("should handle external server references", func() {
			serverRef := &prefectiov1.PrefectServerReference{
				RemoteAPIURL: ptr.To("https://httpbin.org"),
			}

			// This will fail because httpbin.org doesn't have /api/health endpoint
			// But it tests that we don't crash on external servers
			healthy, err := CheckPrefectServerHealth(ctx, serverRef, nil, "default")
			Expect(healthy).To(BeFalse())
			Expect(err).To(HaveOccurred()) // Will error because endpoint doesn't exist
		})

		It("should handle servers with no API URL", func() {
			serverRef := &prefectiov1.PrefectServerReference{}

			healthy, err := CheckPrefectServerHealth(ctx, serverRef, nil, "default")
			Expect(healthy).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to determine API URL"))
		})
	})
})
