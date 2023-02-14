// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clusteraddonsv1alpha1 "github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
)

// ClusterAddonSetReconciler reconciles a ClusterAddon object.
type ClusterAddonSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//nolint:lll // This is a long line.
//+kubebuilder:rbac:groups=clusteraddons.labs.d2iq.io,resources=clusteraddonsets;clusteraddons,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=clusteraddons.labs.d2iq.io,resources=clusteraddonsets/status;clusteraddons/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=clusteraddons.labs.d2iq.io,resources=clusteraddonsets/finalizers;clusteraddons/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterAddon object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ClusterAddonSetReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterAddonSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusteraddonsv1alpha1.ClusterAddonSet{}).
		Owns(&clusteraddonsv1alpha1.ClusterAddon{}).
		Complete(r)
}
