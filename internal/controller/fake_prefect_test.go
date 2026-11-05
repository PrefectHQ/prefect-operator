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
	"context"

	"github.com/go-logr/logr"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/prefect"
	"github.com/PrefectHQ/prefect-operator/internal/prefect/prefecttest"
)

// fakePrefectClients returns a client factory that sends every resource's
// requests to api, whatever server the resource references.
func fakePrefectClients(api *prefecttest.Server) prefect.ClientFactory {
	return func(context.Context, *prefectiov1.PrefectServerReference, string) (*prefect.Client, error) {
		return prefect.NewClient(api.APIURL, "", logr.Discard()), nil
	}
}
