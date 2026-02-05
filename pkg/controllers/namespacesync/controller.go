// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacesync

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type Reconciler struct {
	Client client.Client

	// UnstructuredCachingClient provides a client that forces caching of unstructured objects,
	// optimizing reads of provider-specific resources.
	UnstructuredCachingClient client.Client

	// SourceClusterClassNamespace is the namespace from which ClusterClasses are copied.
	SourceClusterClassNamespace string

	// TargetNamespaceSelector is a label selector to determine which namespaces should receive
	// copies of ClusterClasses and Templates from the source namespace.
	TargetNamespaceSelector labels.Selector
}

func (r *Reconciler) SetupWithManager(
	mgr ctrl.Manager,
	options *controller.Options,
) error {
	if r.TargetNamespaceSelector == nil {
		return fmt.Errorf("TargetNamespaceSelector must be defined to use controller")
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{},
			builder.WithPredicates(
				predicate.Funcs{
					CreateFunc: func(e event.CreateEvent) bool {
						// Called when an object is first seen by the cache, i.e. when a new object is created,
						// or when the cache is populated on start.
						ns, ok := e.Object.(*corev1.Namespace)
						if !ok {
							return false
						}
						return r.TargetNamespaceSelector.Matches(labels.Set(ns.GetLabels()))
					},
					UpdateFunc: func(e event.UpdateEvent) bool {
						// Called when an object is already in the cache, and it is either updated,
						// or fetched as part of a re-list (aka re-sync).
						nsOld, ok := e.ObjectOld.(*corev1.Namespace)
						if !ok {
							return false
						}
						nsNew, ok := e.ObjectNew.(*corev1.Namespace)
						if !ok {
							return false
						}
						// Only reconcile the namespace if the answer to the question "Is this a
						// target namespace?" changed from no to yes.
						matchesOld := r.TargetNamespaceSelector.Matches(labels.Set(nsOld.GetLabels()))
						matchesNew := r.TargetNamespaceSelector.Matches(labels.Set(nsNew.GetLabels()))
						return !matchesOld && matchesNew
					},
					DeleteFunc: func(e event.DeleteEvent) bool {
						// Ignore deletes.
						return false
					},
					GenericFunc: func(e event.GenericEvent) bool {
						// Ignore generic events, i.e. events that don't come from the API server.
						return false
					},
				},
			)).
		Watches(&clusterv1.ClusterClass{},
			handler.EnqueueRequestsFromMapFunc(r.clusterClassToNamespaces),
		).
		Named("syncclusterclass").
		WithOptions(*options).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to set up with controller manager: %w", err)
	}

	return nil
}

func (r *Reconciler) clusterClassToNamespaces(ctx context.Context, o client.Object) []ctrl.Request {
	namespaceList := &corev1.NamespaceList{}
	err := r.Client.List(ctx, namespaceList, &client.ListOptions{
		LabelSelector: r.TargetNamespaceSelector,
	})
	if err != nil {
		// TODO Log the error, and record an Event.
		return nil
	}

	// Pre-allocate slice with exact capacity since we're using label selector
	rs := make([]ctrl.Request, 0, len(namespaceList.Items))
	for i := range namespaceList.Items {
		ns := &namespaceList.Items[i]
		rs = append(rs,
			ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(ns),
			},
		)
	}
	return rs
}

func (r *Reconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (
	_ ctrl.Result,
	rerr error,
) {
	namespace := req.Name

	sccs, err := r.listSourceClusterClasses(ctx)
	if err != nil {
		// TODO Record an Event.
		return ctrl.Result{}, fmt.Errorf("failed to list source ClusterClasses: %w", err)
	}

	// TODO Consider running in parallel.
	for i := range sccs {
		scc := &sccs[i]

		err := copyClusterClassAndTemplates(
			ctx,
			r.Client,
			r.UnstructuredCachingClient,
			scc,
			namespace,
		)
		if err != nil {
			// TODO Record an Event.
			return ctrl.Result{}, fmt.Errorf(
				"failed to copy source ClusterClass %s or its referenced Templates to namespace %s: %w",
				client.ObjectKeyFromObject(scc),
				namespace,
				err,
			)
		}
	}

	// TODO Record an Event.
	return ctrl.Result{}, nil
}

func (r *Reconciler) listSourceClusterClasses(
	ctx context.Context,
) (
	[]clusterv1.ClusterClass,
	error,
) {
	// Handle the empty string explicitly, because listing resources with an empty
	// string namespace returns resources in all namespaces.
	if r.SourceClusterClassNamespace == "" {
		return nil, nil
	}

	ccl := &clusterv1.ClusterClassList{}
	err := r.Client.List(ctx, ccl, client.InNamespace(r.SourceClusterClassNamespace))
	if err != nil {
		return nil, err
	}
	return ccl.Items, nil
}
