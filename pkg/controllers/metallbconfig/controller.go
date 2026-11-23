// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallbconfig

import (
	"context"
	"errors"
	"fmt"
	"maps"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/serviceloadbalancer/metallb"
)

type RemoteClientGetter func(
	ctx context.Context,
	cl client.Client,
	clusterKey client.ObjectKey,
) (client.Client, error)

type Reconciler struct {
	client.Client
	RemoteClientGetter RemoteClientGetter
}

func (r *Reconciler) SetupWithManager(
	mgr ctrl.Manager,
	options *controller.Options,
) error {
	if mgr == nil {
		return errors.New("manager cannot be nil")
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("metallb-config").
		For(&clusterv1.Cluster{}, builder.WithPredicates(clusterPredicate())).
		Watches(
			&caaphv1.HelmChartProxy{},
			handler.EnqueueRequestsFromMapFunc(helmChartProxyToCluster),
			builder.WithPredicates(predicate.NewPredicateFuncs(isMetalLBHelmChartProxy)),
		).
		WithOptions(*options).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx).WithValues("cluster", req.NamespacedName)

	var cluster clusterv1.Cluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cluster.DeletionTimestamp.IsZero() {
		logger.V(5).Info("Cluster is being deleted, skipping MetalLB configuration reconciliation")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(&cluster, &cluster) {
		logger.V(5).Info("Cluster is paused, skipping MetalLB configuration reconciliation")
		return ctrl.Result{}, nil
	}

	if !cluster.Spec.Topology.IsDefined() {
		logger.V(5).Info("Cluster is not using topology, skipping MetalLB configuration reconciliation")
		return ctrl.Result{}, nil
	}

	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)
	slb, err := variables.Get[v1alpha1.ServiceLoadBalancer](
		varMap,
		v1alpha1.ClusterConfigVariableName,
		"addons",
		v1alpha1.ServiceLoadBalancerVariableName,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf(
			"failed to read ServiceLoadBalancer configuration from Cluster %s: %w",
			req.NamespacedName,
			err,
		)
	}

	if slb.Provider != v1alpha1.ServiceLoadBalancerProviderMetalLB || slb.Configuration == nil {
		return ctrl.Result{}, nil
	}

	clusterUUID := cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
	if clusterUUID == "" {
		logger.V(5).Info("Cluster UUID annotation is not set yet, waiting for Cluster update")
		return ctrl.Result{}, nil
	}

	hcpKey := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      metallb.HelmChartProxyName(clusterUUID),
	}
	var hcp caaphv1.HelmChartProxy
	if err := r.Get(ctx, hcpKey, &hcp); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(5).Info(
				"MetalLB HelmChartProxy does not exist yet, waiting for HelmChartProxy event",
				"helmChartProxy",
				hcpKey,
			)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get MetalLB HelmChartProxy %s: %w", hcpKey, err)
	}

	if !isHelmChartProxyReady(&hcp) {
		logger.V(5).Info(
			"MetalLB HelmChartProxy is not ready yet, waiting for Ready condition",
			"helmChartProxy",
			hcpKey,
		)
		return ctrl.Result{}, nil
	}

	remoteClientGetter := r.RemoteClientGetter
	if remoteClientGetter == nil {
		remoteClientGetter = func(
			ctx context.Context,
			cl client.Client,
			clusterKey client.ObjectKey,
		) (client.Client, error) {
			return remote.NewClusterClient(ctx, "", cl, clusterKey)
		}
	}

	remoteClient, err := remoteClientGetter(ctx, r.Client, client.ObjectKeyFromObject(&cluster))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf(
			"failed to create remote cluster client for Cluster %s: %w",
			req.NamespacedName,
			err,
		)
	}

	logger.V(4).Info("Applying MetalLB configuration to workload cluster")
	if err := metallb.ApplyConfiguration(ctx, remoteClient, slb.Configuration); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func clusterPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCluster, okOld := e.ObjectOld.(*clusterv1.Cluster)
			newCluster, okNew := e.ObjectNew.(*clusterv1.Cluster)
			if !okOld || !okNew {
				return false
			}
			if !newCluster.DeletionTimestamp.IsZero() {
				return false
			}
			return newCluster.Generation != oldCluster.Generation ||
				!maps.Equal(newCluster.Annotations, oldCluster.Annotations)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return true
		},
	}
}

func isHelmChartProxyReady(hcp *caaphv1.HelmChartProxy) bool {
	if !hcp.DeletionTimestamp.IsZero() {
		return false
	}
	if hcp.Generation != hcp.Status.ObservedGeneration {
		return false
	}
	return apimeta.IsStatusConditionTrue(hcp.GetConditions(), clusterv1.ReadyCondition)
}

func isMetalLBHelmChartProxy(obj client.Object) bool {
	hcp, ok := obj.(*caaphv1.HelmChartProxy)
	if !ok {
		return false
	}
	return hcp.Spec.ReleaseName == metallb.DefaultHelmReleaseName &&
		hcp.Spec.ReleaseNamespace == metallb.DefaultHelmReleaseNamespace
}

// helmAddonApplier sets a non-controller OwnerReference (Controller == nil), so
// handler.OnlyControllerOwner() cannot be used here.
func helmChartProxyToCluster(_ context.Context, obj client.Object) []reconcile.Request {
	hcp, ok := obj.(*caaphv1.HelmChartProxy)
	if !ok {
		return nil
	}

	for _, ownerRef := range hcp.OwnerReferences {
		if ownerRef.Kind != clusterv1.ClusterKind || ownerRef.Name == "" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		if err == nil && gv.Group == clusterv1.GroupVersion.Group {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Namespace: hcp.Namespace,
					Name:      ownerRef.Name,
				},
			}}
		}
	}

	if clusterName := hcp.Spec.ClusterSelector.MatchLabels[clusterv1.ClusterNameLabel]; clusterName != "" {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Namespace: hcp.Namespace,
				Name:      clusterName,
			},
		}}
	}

	return nil
}
