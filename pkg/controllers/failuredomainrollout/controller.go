// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomainrollout

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
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
	logger.V(5).Info("Starting failure domain rollout reconciliation")

	var cluster clusterv1.Cluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(5).Info("Cluster not found, skipping reconciliation")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Cluster %s: %w", req.NamespacedName, err)
	}

	// Early validation checks
	if cluster.Spec.Topology == nil {
		logger.V(5).Info("Cluster is not using topology, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	if cluster.Spec.ControlPlaneRef == nil {
		logger.V(5).Info("Cluster has no control plane reference, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	if len(cluster.Status.FailureDomains) == 0 {
		logger.V(5).Info("Cluster has no failure domains, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// If the Cluster is not fully reconciled, we should skip our own reconciliation.
	if cluster.Status.ObservedGeneration < cluster.Generation {
		logger.V(5).Info("Cluster is not yet reconciled, skipping failure domain rollout check",
			"observedGeneration", cluster.Status.ObservedGeneration, "generation", cluster.Generation)
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// Get the KubeAdmControlPlane
	kcpKey := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}

	var kcp controlplanev1.KubeadmControlPlane
	if err := r.Get(ctx, kcpKey, &kcp); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(5).Info("KubeAdmControlPlane not found, skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get KubeAdmControlPlane %s: %w", kcpKey, err)
	}

	// If the KubeadmControlPlane is not fully reconciled, we should skip our own reconciliation.
	if kcp.Status.ObservedGeneration < kcp.Generation {
		logger.V(5).Info("KubeAdmControlPlane is not yet reconciled, skipping failure domain rollout check",
			"observedGeneration", kcp.Status.ObservedGeneration, "generation", kcp.Generation)
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// Check if we need to trigger a rollout
	needsRollout, reason, err := r.shouldTriggerRollout(ctx, &cluster, &kcp)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to determine if rollout is needed: %w", err)
	}

	if needsRollout {
		logger.Info("Rollout needed due to failure domain changes", "reason", reason)

		// Check if we should skip the rollout due to recent or ongoing rollout activities
		if shouldSkip, requeueAfter := r.shouldSkipRollout(&kcp); shouldSkip {
			logger.Info("Skipping rollout due to recent or ongoing rollout activity",
				"reason", reason, "requeueAfter", requeueAfter)
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}

		logger.Info("Attempting to trigger KCP rollout due to failure domain changes", "reason", reason)

		// Set rolloutAfter to trigger immediate rollout
		now := metav1.Now()
		kcpCopy := kcp.DeepCopy()
		kcpCopy.Spec.RolloutAfter = &now

		if err := r.Update(ctx, kcpCopy); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update KubeAdmControlPlane %s: %w", kcpKey, err)
		}

		logger.Info(
			"Successfully triggered KCP rollout due to failure domain changes",
			"rolloutAfter",
			now.Format(time.RFC3339),
		)
	}

	return ctrl.Result{}, nil
}

// getMachineDistribution returns the current distribution of machines across failure domains.
func (r *Reconciler) getMachineDistribution(ctx context.Context, cluster *clusterv1.Cluster) (map[string]int, error) {
	var machines clusterv1.MachineList
	if err := r.List(ctx, &machines, client.InNamespace(cluster.Namespace), client.MatchingLabels{
		clusterv1.ClusterNameLabel:         cluster.Name,
		clusterv1.MachineControlPlaneLabel: "",
	}); err != nil {
		return nil, fmt.Errorf("failed to list control plane machines: %w", err)
	}

	distribution := make(map[string]int)
	for i := range machines.Items {
		// Ignore machines that are being deleted.
		if !machines.Items[i].DeletionTimestamp.IsZero() {
			continue
		}
		if machines.Items[i].Spec.FailureDomain != nil {
			distribution[*machines.Items[i].Spec.FailureDomain]++
		}
	}

	return distribution, nil
}

// shouldTriggerRollout determines if a rollout should be triggered based on current failure domain state.
func (r *Reconciler) shouldTriggerRollout(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	kcp *controlplanev1.KubeadmControlPlane,
) (shouldTrigger bool, reason string, err error) {
	logger := ctrl.LoggerFrom(ctx).WithValues("cluster", client.ObjectKeyFromObject(cluster))

	// Get the current machine distribution
	currentDistribution, err := r.getMachineDistribution(ctx, cluster)
	if err != nil {
		return false, "", fmt.Errorf("failed to get current machine distribution: %w", err)
	}

	// Check if any currently used failure domain is disabled or removed
	for usedFD := range currentDistribution {
		if fd, exists := cluster.Status.FailureDomains[usedFD]; !exists {
			logger.V(5).Info("Found removed failure domain in use", "failureDomain", usedFD)
			return true, fmt.Sprintf("failure domain %s is removed", usedFD), nil
		} else if !fd.ControlPlane {
			logger.V(5).Info("Found disabled failure domain in use", "failureDomain", usedFD)
			return true, fmt.Sprintf("failure domain %s is disabled for control plane", usedFD), nil
		}
	}

	// Check if the distribution could be improved
	availableFDs := getAvailableFailureDomains(cluster.Status.FailureDomains)
	if shouldImprove := r.shouldImproveDistribution(kcp, currentDistribution, availableFDs); shouldImprove {
		return true, "failure domain distribution could be improved for better fault tolerance", nil
	}

	return false, "", nil
}

// shouldSkipRollout determines if a rollout should be skipped due to recent or ongoing rollout activities.
// Returns true if rollout should be skipped and the duration to wait before checking again.
func (r *Reconciler) shouldSkipRollout(kcp *controlplanev1.KubeadmControlPlane) (bool, time.Duration) {
	// Check if rollout was triggered recently
	if kcp.Spec.RolloutAfter != nil {
		timeSinceRollout := time.Since(kcp.Spec.RolloutAfter.Time)
		if timeSinceRollout < 15*time.Minute {
			return true, 5 * time.Minute
		}
	}

	// Check if KCP is currently rolling out by comparing updated vs total replicas
	if kcp.Status.UpdatedReplicas < kcp.Status.Replicas {
		return true, 2 * time.Minute
	}

	// Check conditions for ongoing updates
	for _, condition := range kcp.Status.Conditions {
		if condition.Type == controlplanev1.MachinesSpecUpToDateCondition &&
			condition.Status == corev1.ConditionFalse {
			return true, 2 * time.Minute
		}
	}

	return false, 0
}

// shouldImproveDistribution determines if the current failure domain distribution could be improved.
func (r *Reconciler) shouldImproveDistribution(
	kcp *controlplanev1.KubeadmControlPlane,
	currentDistribution map[string]int,
	availableFDs []string,
) bool {
	// Skip if no replicas specified
	if kcp.Spec.Replicas == nil {
		return false
	}

	replicas := int(*kcp.Spec.Replicas)
	availableCount := len(availableFDs)

	// Check if current distribution violates ideal distribution
	if r.hasSuboptimalDistribution(currentDistribution, replicas, availableCount) {
		return true
	}

	// Check if using more failure domains would improve fault tolerance
	currentUsedCount := len(currentDistribution)
	if availableCount > currentUsedCount {
		return r.canImproveWithMoreFDs(currentDistribution, replicas, availableCount)
	}

	return false
}

// hasSuboptimalDistribution checks if any failure domain has more machines than the ideal maximum.
func (r *Reconciler) hasSuboptimalDistribution(distribution map[string]int, replicas, availableCount int) bool {
	maxIdealPerFD := r.calculateMaxIdealPerFD(replicas, availableCount)

	for _, count := range distribution {
		if count > maxIdealPerFD {
			return true
		}
	}

	return false
}

// canImproveWithMoreFDs checks if using additional failure domains would improve fault tolerance by comparing
// current max concentration vs ideal max with optimal distribution.
// Also checks if we can reduce the number of FDs at maximum concentration.
func (r *Reconciler) canImproveWithMoreFDs(currentDistribution map[string]int, replicas, availableCount int) bool {
	if len(currentDistribution) == 0 {
		return false
	}

	// Find current min and max counts to understand current concentration
	minCount, maxCount := replicas, 0
	for _, count := range currentDistribution {
		if count < minCount {
			minCount = count
		}
		if count > maxCount {
			maxCount = count
		}
	}

	// When this function is called, we know hasSuboptimalDistribution was false,
	// which means maxCount <= calculateMaxIdealPerFD(replicas, availableCount).
	// Therefore, we only need to check if we can improve concentration.

	currentFDsAtMax := 0
	for _, count := range currentDistribution {
		if count == maxCount {
			currentFDsAtMax++
		}
	}

	// Calculate optimal number of FDs that should have the maximum
	// In optimal distribution, (replicas % availableCount) FDs get the extra replica
	optimalFDsAtMax := replicas % availableCount
	if optimalFDsAtMax == 0 && replicas > 0 {
		// If evenly divisible and we have replicas, all FDs get the same amount
		// This means no FDs are "at max" in the sense of having extras
		optimalFDsAtMax = availableCount
	}

	// Improvement if we can reduce the number of FDs at maximum concentration
	return optimalFDsAtMax < currentFDsAtMax
}

// calculateMaxIdealPerFD calculates the maximum number of machines per failure domain in ideal distribution.
func (r *Reconciler) calculateMaxIdealPerFD(replicas, availableCount int) int {
	if availableCount == 0 {
		return replicas
	}

	baseReplicasPerFD := replicas / availableCount
	extraReplicas := replicas % availableCount

	if extraReplicas > 0 {
		return baseReplicasPerFD + 1
	}

	return baseReplicasPerFD
}

// getAvailableFailureDomains returns the names of available failure domains for control plane.
func getAvailableFailureDomains(failureDomains clusterv1.FailureDomains) []string {
	var available []string
	for name, fd := range failureDomains {
		if fd.ControlPlane {
			available = append(available, name)
		}
	}
	return available
}

// kubeadmControlPlaneToCluster maps KubeAdmControlPlane changes to cluster reconcile requests.
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

	for i := range clusters.Items {
		if clusters.Items[i].Spec.ControlPlaneRef != nil && clusters.Items[i].Spec.ControlPlaneRef.Name == kcp.Name {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: clusters.Items[i].Namespace,
						Name:      clusters.Items[i].Name,
					},
				},
			}
		}
	}

	return nil
}
