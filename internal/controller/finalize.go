package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// deleteRetryWindow is how long finalize retries a failed Prefect delete,
// measured from the object's deletion timestamp, before it removes the
// finalizer anyway. It is a variable so that tests can shorten it.
var deleteRetryWindow = time.Hour

// deleteRetryInterval is how long finalize waits between Prefect deletes.
const deleteRetryInterval = 30 * time.Second

// finalize runs when obj is being deleted. It calls deleteFromPrefect, unless
// it's nil, and then removes finalizer so that Kubernetes can delete obj.
//
// When deleteFromPrefect fails, finalize keeps the finalizer, records a
// Warning event, and retries every deleteRetryInterval. A failure can be
// temporary, such as a Prefect outage or an API key whose Secret doesn't have
// the prefect.io/allowed-api-urls annotation yet, and removing the finalizer
// would leave the object in Prefect: a deployment there keeps running its
// schedules. Once deleteRetryWindow has passed since obj's deletion
// timestamp, finalize removes the finalizer even if the delete fails, so that
// a delete Prefect always refuses doesn't keep obj in Kubernetes. A Warning
// event on obj then records that the Prefect object may still exist.
func finalize(ctx context.Context, c client.Client, recorder events.EventRecorder, obj client.Object, finalizer string, deleteFromPrefect func(context.Context) error) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		return ctrl.Result{}, nil
	}

	log := log.FromContext(ctx)
	if deleteFromPrefect != nil {
		if err := deleteFromPrefect(ctx); err != nil {
			deadline := deletionTime(obj).Add(deleteRetryWindow)
			if remaining := time.Until(deadline); remaining > 0 {
				log.Error(err, "Failed to delete from Prefect; retrying", "until", deadline)
				recorder.Eventf(obj, nil, corev1.EventTypeWarning, "DeleteFailed", eventActionDelete,
					"Failed to delete from Prefect, retrying until %s: %v", deadline.UTC().Format(time.RFC3339), err)
				return ctrl.Result{RequeueAfter: min(deleteRetryInterval, remaining)}, nil
			}
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

// deletionTime returns obj's deletion timestamp, or the current time if obj
// doesn't have one.
func deletionTime(obj client.Object) time.Time {
	if ts := obj.GetDeletionTimestamp(); ts != nil {
		return ts.Time
	}
	return time.Now()
}
