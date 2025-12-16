// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomainrollout

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
		WithOptions(*options).
		Complete(r)
}

// areResourcesDeleting checks if either the cluster or KCP has a deletion timestamp.
// Returns true if any resource is being deleted and reconciliation should be skipped.
func (r *Reconciler) areResourcesDeleting(cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) bool {
	if cluster != nil && !cluster.DeletionTimestamp.IsZero() {
		return true
	}

	if kcp != nil && !kcp.DeletionTimestamp.IsZero() {
		return true
	}

	return false
}

// areResourcesUpdating checks if either the cluster or KCP has not fully reconciled.
// Returns true if any resource is still being updated and reconciliation should be requeued.
func (r *Reconciler) areResourcesUpdating(cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) bool {
	if cluster != nil && cluster.Status.ObservedGeneration < cluster.Generation {
		return true
	}

	if kcp != nil && kcp.Status.ObservedGeneration < kcp.Generation {
		return true
	}

	return false
}

// areResourcesPaused checks if either the cluster or KCP is paused.
// Uses the standard CAPI annotations.IsPaused utility which handles both cluster.Spec.Paused and paused annotations.
func (r *Reconciler) areResourcesPaused(cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) bool {
	if cluster != nil && annotations.IsPaused(cluster, cluster) {
		return true
	}

	if cluster != nil && kcp != nil && annotations.IsPaused(cluster, kcp) {
		return true
	}

	return false
}

// shouldSkipClusterReconciliation checks if the cluster should be skipped for reconciliation
// based on early validation checks. Returns true if reconciliation should be skipped.
func (r *Reconciler) shouldSkipClusterReconciliation(cluster *clusterv1.Cluster, logger logr.Logger) bool {
	if cluster.Spec.Topology == nil {
		logger.V(5).Info("Cluster is not using topology, skipping reconciliation")
		return true
	}

	if cluster.Spec.ControlPlaneRef == nil {
		logger.V(5).Info("Cluster has no control plane reference, skipping reconciliation")
		return true
	}

	if len(cluster.Status.FailureDomains) == 0 {
		logger.V(5).Info("Cluster has no failure domains, skipping reconciliation")
		return true
	}

	return false
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
	if r.shouldSkipClusterReconciliation(&cluster, logger) {
		return ctrl.Result{}, nil
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

	// Skip reconciliation if either cluster or KCP is being deleted
	if r.areResourcesDeleting(&cluster, &kcp) {
		logger.V(5).Info("Cluster or KubeadmControlPlane is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Skip reconciliation if either cluster or KCP is paused
	if r.areResourcesPaused(&cluster, &kcp) {
		logger.V(5).Info("Cluster or KubeadmControlPlane is paused, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Skip reconciliation if either cluster or KCP is not fully reconciled
	if r.areResourcesUpdating(&cluster, &kcp) {
		logger.V(5).Info("Cluster or KubeadmControlPlane is not yet reconciled, requeuing")
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// Check if we need to trigger a rollout
	needsRollout, reason, err := r.shouldTriggerRollout(ctx, &cluster, &kcp)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to determine if rollout is needed: %w", err)
	}

	if !needsRollout {
		logger.V(5).Info("No rollout needed due to failure domain changes", "reason", reason)
		return ctrl.Result{}, nil
	}

	logger.Info("Rollout needed due to failure domain changes", "reason", reason)

	// Check if we should skip the rollout due to recent or ongoing rollout activities
	if shouldSkip, requeueAfter := r.shouldSkipRollout(&kcp); shouldSkip {
		logger.V(5).Info("Skipping rollout due to recent or ongoing rollout activity",
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

// canImproveWithMoreFDs checks if using additional failure domains would improve fault tolerance
// by reducing either the maximum number of replicas per FD, or the number of FDs at maximum concentration.
func (r *Reconciler) canImproveWithMoreFDs(currentDistribution map[string]int, replicas, availableCount int) bool {
	if len(currentDistribution) == 0 || replicas == 0 || availableCount <= len(currentDistribution) {
		return false
	}

	currentMaxPerFD, currentFDsAtMax := 0, 0
	for _, count := range currentDistribution {
		if count > currentMaxPerFD {
			currentMaxPerFD, currentFDsAtMax = count, 1
		} else if count == currentMaxPerFD {
			currentFDsAtMax++
		}
	}

	optimalMaxPerFD := r.calculateMaxIdealPerFD(replicas, availableCount)
	extra := replicas % availableCount
	optimalFDsAtMax := extra
	if optimalFDsAtMax == 0 {
		optimalFDsAtMax = availableCount
	}

	return optimalMaxPerFD < currentMaxPerFD ||
		(optimalMaxPerFD == currentMaxPerFD && optimalFDsAtMax < currentFDsAtMax)
}

// calculateMaxIdealPerFD calculates the maximum number of machines per failure domain in ideal distribution.
func (r *Reconciler) calculateMaxIdealPerFD(replicas, availableCount int) int {
	if availableCount == 0 {
		return replicas
	}
	base := replicas / availableCount
	if replicas%availableCount > 0 {
		return base + 1
	}
	return base
}

// getAvailableFailureDomains returns the names of available failure domains for control plane.
func getAvailableFailureDomains(failureDomains clusterv1.FailureDomains) []string {
	var available []string
	for fd, info := range failureDomains {
		if info.ControlPlane {
			available = append(available, fd)
		}
	}
	return available
}
