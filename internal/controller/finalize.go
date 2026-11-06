package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// finalize runs when obj is being deleted. It calls deleteFromPrefect, unless
// it's nil, and then removes finalizer so that Kubernetes can delete obj.
//
// The finalizer is removed even when deleteFromPrefect fails. Otherwise a
// Prefect outage, or a Prefect object that Prefect won't delete, such as a
// work pool's default queue, would keep obj in Kubernetes until someone
// removed the finalizer by hand. A Warning event on obj records that the
// Prefect object may still exist.
func finalize(ctx context.Context, c client.Client, recorder events.EventRecorder, obj client.Object, finalizer string, deleteFromPrefect func(context.Context) error) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		return ctrl.Result{}, nil
	}

	log := log.FromContext(ctx)
	if deleteFromPrefect != nil {
		if err := deleteFromPrefect(ctx); err != nil {
			log.Error(err, "Failed to delete from Prefect; removing the finalizer anyway")
			recorder.Eventf(obj, nil, corev1.EventTypeWarning, "DeleteFailed", eventActionDelete,
				"Failed to delete from Prefect, so it may still exist there: %v", err)
		} else {
			log.Info("Deleted from Prefect")
		}
	}

	controllerutil.RemoveFinalizer(obj, finalizer)
	if err := c.Update(ctx, obj); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
	}
	log.Info("Removed finalizer")
	return ctrl.Result{}, nil
}
