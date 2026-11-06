package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// requestsFor lists every object of list's type in the watched namespaces and
// returns a reconcile request for each one that matches.
func requestsFor[T client.Object](ctx context.Context, c client.Client, list client.ObjectList, matches func(T) bool) []reconcile.Request {
	if err := c.List(ctx, list); err != nil {
		log.FromContext(ctx).Error(err, "Failed to list resources to reconcile")
		return nil
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to read listed resources")
		return nil
	}

	var requests []reconcile.Request
	for _, item := range items {
		if obj, ok := item.(T); ok && matches(obj) {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(obj)})
		}
	}
	return requests
}

// gotPrefectID passes only updates that set or change an object's Prefect ID,
// as read by id. The ID changes when the object's first sync creates or adopts
// it in Prefect, and when a sync recreates it there.
func gotPrefectID[T client.Object](id func(T) *string) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj, okOld := e.ObjectOld.(T)
			newObj, okNew := e.ObjectNew.(T)
			if !okOld || !okNew {
				return false
			}
			oldID, newID := id(oldObj), id(newObj)
			if newID == nil || *newID == "" {
				return false
			}
			return oldID == nil || *oldID != *newID
		},
	}
}

// workPoolGotPrefectID passes PrefectWorkPool updates that set or change its
// Prefect ID.
var workPoolGotPrefectID = gotPrefectID(func(p *prefectiov1.PrefectWorkPool) *string { return p.Status.Id })

// deploymentGotPrefectID passes PrefectDeployment updates that set or change
// its Prefect ID.
var deploymentGotPrefectID = gotPrefectID(func(d *prefectiov1.PrefectDeployment) *string { return d.Status.Id })
