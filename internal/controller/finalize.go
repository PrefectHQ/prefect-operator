package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/PrefectHQ/prefect-operator/internal/prefect"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

// deleteRetryInterval is the longest that finalize waits between Prefect
// deletes. The PrefectServer finalizer also checks its dependents this often.
const deleteRetryInterval = 30 * time.Second

// deleteRetryMinInterval is the shortest that finalize waits between Prefect
// deletes.
const deleteRetryMinInterval = time.Second

// finalize runs when obj is being deleted. It calls deleteFromPrefect, unless
// it's nil, and then removes finalizer so that Kubernetes can delete obj.
//
// When deleteFromPrefect fails, finalize keeps the finalizer, records a
// Warning event, and retries after deleteRetryDelay. A failure can be
// temporary, such as a Prefect outage or an API key whose Secret doesn't have
// the prefect.io/allowed-api-urls annotation yet, and removing the finalizer
// would leave the object in Prefect: a deployment there keeps running its
// schedules. Once deleteRetryWindow has passed since obj's deletion
// timestamp, finalize removes the finalizer even if the delete fails, so that
// a delete Prefect always refuses doesn't keep obj in Kubernetes. A Warning
// event on obj then records that the Prefect object may still exist.
//
// finalize also removes the finalizer after the first failed delete when
// Kubernetes deleted what the delete needs along with a terminating namespace,
// as goneWithNamespace reports. Nothing can create objects in a terminating
// namespace again, so retrying would only hold up the namespace's deletion.
func finalize(ctx context.Context, c client.Client, recorder events.EventRecorder, obj client.Object, finalizer string, deleteFromPrefect func(context.Context) error) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		return ctrl.Result{}, nil
	}

	log := log.FromContext(ctx)
	if deleteFromPrefect != nil {
		if err := deleteFromPrefect(ctx); err != nil {
			if gone := goneWithNamespace(ctx, c, obj); gone != "" {
				log.Error(err, "Failed to delete from Prefect; removing the finalizer, because the delete needs an object that was deleted with its namespace", "deleted", gone)
				recorder.Eventf(obj, nil, corev1.EventTypeWarning, "DeleteFailed", eventActionDelete,
					"Failed to delete from Prefect, and %s was deleted with its namespace, so it may still exist there: %v", gone, err)
				return removeFinalizer(ctx, c, obj, finalizer)
			}
			deadline := deletionTime(obj).Add(deleteRetryWindow)
			if remaining := time.Until(deadline); remaining > 0 {
				log.Error(err, "Failed to delete from Prefect; retrying", "until", deadline)
				recorder.Eventf(obj, nil, corev1.EventTypeWarning, "DeleteFailed", eventActionDelete,
					"Failed to delete from Prefect, retrying until %s: %v", deadline.UTC().Format(time.RFC3339), err)
				return ctrl.Result{RequeueAfter: min(deleteRetryDelay(time.Since(deletionTime(obj))), remaining)}, nil
			}
			log.Error(err, "Failed to delete from Prefect; removing the finalizer anyway")
			recorder.Eventf(obj, nil, corev1.EventTypeWarning, "DeleteFailed", eventActionDelete,
				"Failed to delete from Prefect, so it may still exist there: %v", err)
		} else {
			log.Info("Deleted from Prefect")
		}
	}

	return removeFinalizer(ctx, c, obj, finalizer)
}

func removeFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(obj, finalizer)
	if err := c.Update(ctx, obj); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
	}
	log.FromContext(ctx).Info("Removed finalizer")
	return ctrl.Result{}, nil
}

// goneWithNamespace returns the kind, namespace, and name of an object that
// deleting obj from Prefect needs and that is missing from a terminating
// namespace: the Secret or ConfigMap with obj's API key or auth string, or the
// Service of the in-cluster server that obj uses. It returns "" when obj has
// no spec.server, or when each object exists or is in a namespace that isn't
// terminating, where someone can still create it. An error reading an object
// counts as the object existing.
func goneWithNamespace(ctx context.Context, c client.Client, obj client.Object) string {
	user, ok := obj.(serverUser)
	if !ok {
		return ""
	}
	ref := user.ServerReference()
	namespace := obj.GetNamespace()

	type needed struct {
		obj  client.Object
		kind string
		key  types.NamespacedName
	}
	var objects []needed
	if ref.APIKey != nil && ref.APIKey.ValueFrom != nil {
		if s := ref.APIKey.ValueFrom.SecretKeyRef; s != nil {
			objects = append(objects, needed{&corev1.Secret{}, "Secret", types.NamespacedName{Namespace: namespace, Name: s.Name}})
		}
		if cm := ref.APIKey.ValueFrom.ConfigMapKeyRef; cm != nil {
			objects = append(objects, needed{&corev1.ConfigMap{}, "ConfigMap", types.NamespacedName{Namespace: namespace, Name: cm.Name}})
		}
	}
	if ref.Auth != nil {
		objects = append(objects, needed{&corev1.Secret{}, "Secret", types.NamespacedName{Namespace: namespace, Name: ref.Auth.SecretKeyRef.Name}})
	}
	if ref.IsInCluster() && !ref.IsRemote() {
		objects = append(objects, needed{&corev1.Service{}, "Service", types.NamespacedName{Namespace: ref.ServerNamespace(namespace), Name: ref.Name}})
	}

	terminating := map[string]bool{}
	for _, n := range objects {
		isTerminating, checked := terminating[n.key.Namespace]
		if !checked {
			isTerminating = namespaceTerminating(ctx, c, n.key.Namespace)
			terminating[n.key.Namespace] = isTerminating
		}
		if isTerminating && apierrors.IsNotFound(c.Get(ctx, n.key, n.obj)) {
			return fmt.Sprintf("%s %s/%s", n.kind, n.key.Namespace, n.key.Name)
		}
	}
	return ""
}

//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get

// namespaceTerminating reports whether the namespace has been deleted. It
// returns false if the namespace can't be read.
func namespaceTerminating(ctx context.Context, c client.Client, name string) bool {
	namespace := &corev1.Namespace{}
	if err := c.Get(ctx, types.NamespacedName{Name: name}, namespace); err != nil {
		return false
	}
	return namespace.DeletionTimestamp != nil || namespace.Status.Phase == corev1.NamespaceTerminating
}

// UncachedObjects returns the kinds that the reconcilers' client must read
// from the API server instead of its cache. Pass it as
// client.CacheOptions.DisableFor. It adds Namespaces to
// prefect.UncachedObjects, because the operator has get on Namespaces but not
// list or watch.
func UncachedObjects() []client.Object {
	return append(prefect.UncachedObjects(), &corev1.Namespace{})
}

// deleteRetryDelay returns how long finalize waits before it retries a failed
// Prefect delete, given the time since the object's deletion timestamp. The
// delay equals that time, between deleteRetryMinInterval and
// deleteRetryInterval, so it about doubles with each retry. A short failure,
// such as a 503 from an ephemeral server that another delete is writing to,
// then costs about a second instead of deleteRetryInterval.
func deleteRetryDelay(sinceDeletion time.Duration) time.Duration {
	return min(max(sinceDeletion, deleteRetryMinInterval), deleteRetryInterval)
}

// deletionTime returns obj's deletion timestamp, or the current time if obj
// doesn't have one.
func deletionTime(obj client.Object) time.Time {
	if ts := obj.GetDeletionTimestamp(); ts != nil {
		return ts.Time
	}
	return time.Now()
}
