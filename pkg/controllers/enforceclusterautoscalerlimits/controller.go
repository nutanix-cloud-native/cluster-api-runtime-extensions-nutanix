// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package enforceclusterautoscalerlimits

import (
	context "context"
	fmt "fmt"
	"strconv"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
		For(&clusterv1.MachineDeployment{}).
		WithOptions(*options).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx).WithValues("machineDeployment", req.NamespacedName)

	var md clusterv1.MachineDeployment
	if err := r.Get(ctx, req.NamespacedName, &md); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(5).Info("MachineDeployment not found, skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get MachineDeployment %s: %w", req.NamespacedName, err)
	}

	// If replicas is not set, we don't need to do anything.
	if md.Spec.Replicas == nil {
		logger.V(5).Info("MachineDeployment has no replicas set, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	minReplicas, err := minReplicasFromAnnotations(md.Annotations)
	if err != nil {
		// Do nothing if the minSize annotation is missing.
		if errors.Is(err, errMissingMinAnnotation) {
			logger.V(5).Info("MachineDeployment has no min size annotation, skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get min size: %w", err)
	}

	maxReplicas, err := maxReplicasFromAnnotations(md.Annotations)
	if err != nil {
		// Do nothing if the maxSize annotation is missing.
		if errors.Is(err, errMissingMaxAnnotation) {
			logger.V(5).Info("MachineDeployment has no max size annotation, skipping reconciliation")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get max size: %w", err)
	}

	if minReplicas > maxReplicas {
		logger.WithValues("minReplicas", minReplicas, "maxReplicas", maxReplicas).
			Info("Min replicas is greater than max replicas - skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// If the current replicas are within the bounds, do nothing.
	if int(*md.Spec.Replicas) >= minReplicas && int(*md.Spec.Replicas) <= maxReplicas {
		return ctrl.Result{}, nil
	}

	// Otherwise set replicas to nil and depend on CAPI MachineDeployment defaulting to handle
	// the scaling correctly.
	// See https://github.com/kubernetes-sigs/cluster-api/blob/v1.10.3/internal/webhooks/machinedeployment.go#L365
	// for more details.
	md.Spec.Replicas = nil

	if err := r.Update(ctx, &md); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update MachineDeployment %s: %w", req.NamespacedName, err)
	}

	return ctrl.Result{}, nil
}

var (
	// errMissingMinAnnotation is the error returned when a
	// machine set does not have an annotation keyed by
	// nodeGroupMinSizeAnnotationKey.
	errMissingMinAnnotation = errors.New("missing min annotation")

	// errMissingMaxAnnotation is the error returned when a
	// machine set does not have an annotation keyed by
	// nodeGroupMaxSizeAnnotationKey.
	errMissingMaxAnnotation = errors.New("missing max annotation")

	// errInvalidMinAnnotationValue is the error returned when a
	// machine set has a non-integral min annotation value.
	errInvalidMinAnnotation = errors.New("invalid min annotation")

	// errInvalidMaxAnnotationValue is the error returned when a
	// machine set has a non-integral max annotation value.
	errInvalidMaxAnnotation = errors.New("invalid max annotation")
)

// minReplicasFromAnnotations returns the minimum value encoded in the annotations keyed
// by "cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size".
// Returns errMissingMinAnnotation if the annotation doesn't exist or
// errInvalidMinAnnotation if the value is not of type int.
func minReplicasFromAnnotations(annotations map[string]string) (int, error) {
	val, found := annotations[clusterv1.AutoscalerMinSizeAnnotation]
	if !found {
		return 0, errMissingMinAnnotation
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidMinAnnotation, err)
	}
	return i, nil
}

// maxReplicasFromAnnotations returns the maximum value encoded in the annotations keyed
// by "cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size".
// Returns errMissingMaxAnnotation if the annotation doesn't exist or
// errInvalidMaxAnnotation if the value is not of type int.
func maxReplicasFromAnnotations(annotations map[string]string) (int, error) {
	val, found := annotations[clusterv1.AutoscalerMaxSizeAnnotation]
	if !found {
		return 0, errMissingMaxAnnotation
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidMaxAnnotation, err)
	}
	return i, nil
}
