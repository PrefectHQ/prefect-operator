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
