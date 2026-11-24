package controller

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/conditions"
)

// PrefectServerFinalizer keeps a PrefectServer, and so its Deployment and
// Service, until no resource's spec.server names it. Without it, the
// resources that use the server can't reach its API to delete their Prefect
// objects when they are deleted along with it.
const PrefectServerFinalizer = "prefect.io/server-cleanup"

// maxDependentsInCondition is how many dependents the DeletionBlocked
// condition names. It reports the total either way.
const maxDependentsInCondition = 5

// serverDependentKind is a kind whose spec.server can name an in-cluster
// PrefectServer: an empty object of the kind, and an empty list of it.
type serverDependentKind struct {
	object serverUser
	list   client.ObjectList
}

// serverDependentKinds returns each kind whose spec.server can name an
// in-cluster PrefectServer.
func serverDependentKinds() []serverDependentKind {
	return []serverDependentKind{
		{&prefectiov1.PrefectWorkPool{}, &prefectiov1.PrefectWorkPoolList{}},
		{&prefectiov1.PrefectWorkQueue{}, &prefectiov1.PrefectWorkQueueList{}},
		{&prefectiov1.PrefectDeployment{}, &prefectiov1.PrefectDeploymentList{}},
		{&prefectiov1.PrefectAutomation{}, &prefectiov1.PrefectAutomationList{}},
	}
}

// serverUser is an object with a spec.server.
type serverUser interface {
	client.Object
	ServerReference() *prefectiov1.PrefectServerReference
}

// serverOf returns the namespace and name of the in-cluster PrefectServer that
// obj's spec.server names. ok is false when obj has no spec.server or sets
// remoteApiUrl, which GetAPIURL uses even when spec.server also has a name.
func serverOf(obj client.Object) (key types.NamespacedName, ok bool) {
	user, ok := obj.(serverUser)
	if !ok {
		return types.NamespacedName{}, false
	}
	ref := user.ServerReference()
	if !ref.IsInCluster() || ref.IsRemote() {
		return types.NamespacedName{}, false
	}
	return types.NamespacedName{Namespace: ref.ServerNamespace(obj.GetNamespace()), Name: ref.Name}, true
}

// serverDependents returns "<Kind> <namespace>/<name>" for each resource in
// the watched namespaces whose spec.server names server, sorted. Resources in
// namespaces the operator doesn't watch are not listed. The operator never
// added its finalizer to them, so their deletion doesn't need the server.
func (r *PrefectServerReconciler) serverDependents(ctx context.Context, server *prefectiov1.PrefectServer) ([]string, error) {
	serverKey := client.ObjectKeyFromObject(server)
	var dependents []string
	for _, kind := range serverDependentKinds() {
		list := kind.list
		if err := r.List(ctx, list); err != nil {
			return nil, fmt.Errorf("listing %T: %w", list, err)
		}
		items, err := meta.ExtractList(list)
		if err != nil {
			return nil, fmt.Errorf("reading %T: %w", list, err)
		}
		for _, item := range items {
			obj, ok := item.(client.Object)
			if !ok {
				continue
			}
			if key, ok := serverOf(obj); !ok || key != serverKey {
				continue
			}
			gvk, err := apiutil.GVKForObject(obj, r.Scheme)
			if err != nil {
				return nil, err
			}
			dependents = append(dependents, fmt.Sprintf("%s %s", gvk.Kind, client.ObjectKeyFromObject(obj)))
		}
	}
	slices.Sort(dependents)
	return dependents, nil
}

// handleDeletion removes the server's finalizer once no resource uses the
// server. Until then it keeps the finalizer, sets the DeletionBlocked
// condition, and checks again every deleteRetryInterval. A watch on the
// dependents also requeues the server when one of them is deleted.
//
// The server waits for every dependent, including ones nobody is deleting,
// the way Kubernetes keeps a PersistentVolumeClaim that a pod uses. Waiting
// only for dependents that are being deleted would race with a command like
// `kubectl delete -f server.yaml -f work-pool.yaml`, which can delete the
// server before the work pool has a deletion timestamp.
func (r *PrefectServerReconciler) handleDeletion(ctx context.Context, server *prefectiov1.PrefectServer) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(server, PrefectServerFinalizer) {
		return ctrl.Result{}, nil
	}

	dependents, err := r.serverDependents(ctx, server)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(dependents) == 0 {
		return finalize(ctx, r.Client, r.Recorder, server, PrefectServerFinalizer, nil)
	}

	condition := conditions.DeletionBlocked(dependents[:min(len(dependents), maxDependentsInCondition)], len(dependents))
	if meta.SetStatusCondition(&server.Status.Conditions, condition) {
		log.FromContext(ctx).Info("Waiting for resources that use the server to be deleted", "dependents", dependents)
		r.Recorder.Eventf(server, nil, corev1.EventTypeWarning, condition.Reason, eventActionDelete, "%s", condition.Message)
		if err := r.Status().Update(ctx, server); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
		}
	}
	return ctrl.Result{RequeueAfter: deleteRetryInterval}, nil
}

// serverOfDeletedDependent returns a request for the PrefectServer that obj
// uses, if that server is being deleted. It reads the server from the
// manager's cache, which also drops servers in namespaces the operator doesn't
// watch: the cache returns an error for those.
func (r *PrefectServerReconciler) serverOfDeletedDependent(ctx context.Context, obj client.Object) []reconcile.Request {
	key, ok := serverOf(obj)
	if !ok {
		return nil
	}
	server := &prefectiov1.PrefectServer{}
	if err := r.Get(ctx, key, server); err != nil || server.DeletionTimestamp == nil {
		return nil
	}
	return []reconcile.Request{{NamespacedName: key}}
}

// dependentDeleted passes only delete events.
var dependentDeleted = predicate.Funcs{
	CreateFunc:  func(event.CreateEvent) bool { return false },
	UpdateFunc:  func(event.UpdateEvent) bool { return false },
	GenericFunc: func(event.GenericEvent) bool { return false },
	DeleteFunc:  func(event.DeleteEvent) bool { return true },
}
