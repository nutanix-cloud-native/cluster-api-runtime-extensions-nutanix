// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomainrollout

import (
	"context"
	"fmt"
	"sort"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// FailureDomainLastUpdateAnnotation is the annotation key used to store the last failure domain update time
	FailureDomainLastUpdateAnnotation = "caren.nutanix.com/failure-domain-last-update"
	// FailureDomainHashAnnotation is the annotation key used to store the hash of the failure domains
	FailureDomainHashAnnotation = "caren.nutanix.com/failure-domain-hash"
)

type Reconciler struct {
	client.Client
}

func (r *Reconciler) SetupWithManager(
	mgr ctrl.Manager,
	options *controller.Options,
) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		Watches(
			&controlplanev1.KubeadmControlPlane{},
			handler.EnqueueRequestsFromMapFunc(r.kubeadmControlPlaneToCluster),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		WithOptions(*options).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx).WithValues("cluster", req.NamespacedName)

	var cluster clusterv1.Cluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(5).Info("Cluster not found, skipping reconciliation")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Cluster %s: %w", req.NamespacedName, err)
	}

	// Skip if cluster is not using topology
	if cluster.Spec.Topology == nil {
		logger.V(5).Info("Cluster is not using topology, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Skip if cluster doesn't have a control plane reference
	if cluster.Spec.ControlPlaneRef == nil {
		logger.V(5).Info("Cluster has no control plane reference, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Skip if cluster doesn't have failure domains
	if cluster.Status.FailureDomains == nil || len(cluster.Status.FailureDomains) == 0 {
		logger.V(5).Info("Cluster has no failure domains, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Get the KubeAdmControlPlane
	var kcp controlplanev1.KubeadmControlPlane
	kcpKey := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}
	if err := r.Get(ctx, kcpKey, &kcp); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(5).Info("KubeAdmControlPlane not found, skipping reconciliation")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get KubeAdmControlPlane %s: %w", kcpKey, err)
	}

	// Check if we need to trigger a rollout
	needsRollout, reason, err := r.shouldTriggerRollout(ctx, &cluster, &kcp)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to determine if rollout is needed: %w", err)
	}

	if needsRollout {
		logger.Info("Triggering rollout due to failure domain changes", "reason", reason)

		// Set rolloutAfter to trigger immediate rollout
		now := metav1.Now()
		kcpCopy := kcp.DeepCopy()
		kcpCopy.Spec.RolloutAfter = &now

		// Update the annotation to track the last update
		if kcpCopy.Annotations == nil {
			kcpCopy.Annotations = make(map[string]string)
		}
		kcpCopy.Annotations[FailureDomainLastUpdateAnnotation] = now.Format(time.RFC3339)

		// Store the current failure domain hash
		fdHash := r.calculateFailureDomainHash(cluster.Status.FailureDomains)
		kcpCopy.Annotations[FailureDomainHashAnnotation] = fdHash

		if err := r.Update(ctx, kcpCopy); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update KubeAdmControlPlane %s: %w", kcpKey, err)
		}

		logger.Info("Successfully triggered rollout", "rolloutAfter", now.Format(time.RFC3339))
	}

	return ctrl.Result{}, nil
}

// shouldTriggerRollout determines if a rollout should be triggered based on failure domain changes
func (r *Reconciler) shouldTriggerRollout(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	kcp *controlplanev1.KubeadmControlPlane,
) (bool, string, error) {
	logger := ctrl.LoggerFrom(ctx).WithValues("cluster", client.ObjectKeyFromObject(cluster))

	// Calculate current failure domain hash
	currentFDHash := r.calculateFailureDomainHash(cluster.Status.FailureDomains)

	// Get the stored hash from annotations
	storedFDHash, hasStoredHash := kcp.Annotations[FailureDomainHashAnnotation]

	// If no stored hash, this is the first time - store the hash but don't trigger rollout
	if !hasStoredHash {
		logger.V(5).Info("No previous failure domain hash found, storing current hash")
		return false, "", nil
	}

	// If hashes are the same, no changes detected
	if currentFDHash == storedFDHash {
		logger.V(5).Info("No failure domain changes detected")
		return false, "", nil
	}

	// Get the current KCP machines to understand current placement
	currentlyUsedFailureDomains, err := r.getCurrentlyUsedFailureDomains(ctx, cluster, kcp)
	if err != nil {
		return false, "", fmt.Errorf("failed to get currently used failure domains: %w", err)
	}

	logger.V(5).Info("Analyzing failure domain changes",
		"currentlyUsedFailureDomains", currentlyUsedFailureDomains,
		"availableFailureDomains", getAvailableFailureDomains(cluster.Status.FailureDomains))

	// Check if any currently used failure domain is disabled or removed
	for _, usedFD := range currentlyUsedFailureDomains {
		if fd, exists := cluster.Status.FailureDomains[usedFD]; !exists {
			return true, fmt.Sprintf("failure domain %s is removed", usedFD), nil
		} else if !fd.ControlPlane {
			return true, fmt.Sprintf("failure domain %s is disabled for control plane", usedFD), nil
		}
	}

	// If we reach here, failure domains changed but no meaningful impact
	// (e.g., new failure domains added but existing ones still valid)
	logger.V(5).Info("Failure domains changed but no meaningful impact detected")
	return false, "", nil
}

// getCurrentlyUsedFailureDomains returns the failure domains currently used by the KCP machines
func (r *Reconciler) getCurrentlyUsedFailureDomains(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	kcp *controlplanev1.KubeadmControlPlane,
) ([]string, error) {
	var machines clusterv1.MachineList
	if err := r.List(ctx, &machines, client.InNamespace(cluster.Namespace), client.MatchingLabels{
		clusterv1.ClusterNameLabel:         cluster.Name,
		clusterv1.MachineControlPlaneLabel: "",
	}); err != nil {
		return nil, fmt.Errorf("failed to list control plane machines: %w", err)
	}

	usedFailureDomains := make(map[string]struct{})
	for _, machine := range machines.Items {
		if machine.Spec.FailureDomain != nil {
			usedFailureDomains[*machine.Spec.FailureDomain] = struct{}{}
		}
	}

	result := make([]string, 0, len(usedFailureDomains))
	for fd := range usedFailureDomains {
		result = append(result, fd)
	}
	return result, nil
}

// calculateFailureDomainHash calculates a hash of the failure domains for comparison
func (r *Reconciler) calculateFailureDomainHash(failureDomains clusterv1.FailureDomains) string {
	if len(failureDomains) == 0 {
		return ""
	}

	// Create a deterministic representation of the failure domains
	// focusing on control plane eligible domains
	var controlPlaneDomains []string
	for name, fd := range failureDomains {
		if fd.ControlPlane {
			controlPlaneDomains = append(controlPlaneDomains, name)
		}
	}

	// Sort to ensure deterministic hash
	sort.Strings(controlPlaneDomains)

	// Use a simple concatenation for now - in production, consider using a proper hash function
	result := ""
	for _, name := range controlPlaneDomains {
		result += name + ","
	}
	return result
}

// getAvailableFailureDomains returns the names of available failure domains for control plane
func getAvailableFailureDomains(failureDomains clusterv1.FailureDomains) []string {
	var available []string
	for name, fd := range failureDomains {
		if fd.ControlPlane {
			available = append(available, name)
		}
	}
	return available
}

// kubeadmControlPlaneToCluster maps KubeAdmControlPlane changes to cluster reconcile requests
func (r *Reconciler) kubeadmControlPlaneToCluster(ctx context.Context, obj client.Object) []reconcile.Request {
	kcp, ok := obj.(*controlplanev1.KubeadmControlPlane)
	if !ok {
		return nil
	}

	// Find the cluster that owns this KCP
	var clusters clusterv1.ClusterList
	if err := r.List(ctx, &clusters, client.InNamespace(kcp.Namespace)); err != nil {
		return nil
	}

	for _, cluster := range clusters.Items {
		if cluster.Spec.ControlPlaneRef != nil && cluster.Spec.ControlPlaneRef.Name == kcp.Name {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: cluster.Namespace,
						Name:      cluster.Name,
					},
				},
			}
		}
	}

	return nil
}
