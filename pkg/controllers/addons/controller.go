// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
)

type Reconciler struct {
	client client.Client

	handlers []lifecycle.OnClusterSpecUpdated
}

func NewController(cl client.Client, lifecycleHandlers []handlers.Named) *Reconciler {
	specUpdatedHandlers := []lifecycle.OnClusterSpecUpdated{}
	for _, h := range lifecycleHandlers {
		if h, ok := h.(lifecycle.OnClusterSpecUpdated); ok {
			specUpdatedHandlers = append(specUpdatedHandlers, h)
		}
	}
	return &Reconciler{
		client:   cl,
		handlers: specUpdatedHandlers,
	}
}

func (r *Reconciler) SetupWithManager(
	ctx context.Context,
	mgr ctrl.Manager,
	options controller.Options,
) error {
	hasTopologyPredicate := predicates.ClusterHasTopology(ctrl.LoggerFrom(ctx))
	generationChangedPredicate := predicate.GenerationChangedPredicate{}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}, builder.WithPredicates(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					// Only reconcile Cluster with topology.
					if !hasTopologyPredicate.UpdateFunc(e) {
						return false
					}
					if !generationChangedPredicate.Update(e) {
						return false
					}
					cluster, ok := e.ObjectNew.(*clusterv1.Cluster)
					if !ok {
						return false
					}

					return !cluster.Spec.Paused
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			},
		)).
		Named("addons").
		WithOptions(options).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to set up with controller manager: %w", err)
	}

	return nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cluster := &clusterv1.Cluster{}
	if err := r.client.Get(ctx, req.NamespacedName, cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for _, h := range r.handlers {
		if err := h.OnClusterSpecUpdated(ctx, cluster); err != nil {
			return ctrl.Result{}, fmt.Errorf(
				"failed to reconcile cluster %s: %w",
				client.ObjectKeyFromObject(cluster),
				err,
			)
		}
	}

	return ctrl.Result{}, nil
}
