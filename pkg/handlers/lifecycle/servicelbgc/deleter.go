// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelbgc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/api/core/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers"
)

const (
	LoadBalancerGCAnnotation = handlers.MetadataDomain + "/loadbalancer-gc"
)

var (
	ErrFailedToDeleteService = errors.New("failed to delete kubernetes services")
	ErrServicesStillExist    = errors.New("waiting for kubernetes services to be fully deleted")
)

func deleteServicesWithLoadBalancer(
	ctx context.Context,
	c ctrlclient.Client,
	log logr.Logger,
) error {
	log.Info("Listing Services with type LoadBalancer")
	services := &corev1.ServiceList{}
	err := c.List(ctx, services)
	if err != nil {
		return fmt.Errorf("error listing Services: %w", err)
	}

	var (
		svcsFailedToBeDeleted []ctrlclient.ObjectKey
		svcsStillExisting     []ctrlclient.ObjectKey
	)
	for idx := range services.Items {
		svc := &services.Items[idx]
		svcKey := ctrlclient.ObjectKeyFromObject(svc)
		if needsDelete(svc) {
			svcsStillExisting = append(svcsStillExisting, svcKey)

			if svc.DeletionTimestamp != nil {
				continue
			}

			log.Info(fmt.Sprintf("Deleting Service %s", svcKey))
			if err = c.Delete(ctx, svc); err != nil {
				if ctrlclient.IgnoreNotFound(err) == nil {
					continue
				}
				log.Error(
					err,
					fmt.Sprintf(
						"Error deleting Service %s/%s",
						svc.Namespace,
						svc.Name,
					),
				)
				svcsFailedToBeDeleted = append(svcsFailedToBeDeleted, svcKey)
			}
		}
	}
	if len(svcsFailedToBeDeleted) > 0 {
		return failedToDeleteServicesError(svcsFailedToBeDeleted)
	}
	if len(svcsStillExisting) > 0 {
		return servicesStillExistError(svcsStillExisting)
	}

	return nil
}

// needsDelete will return true if the Service needs to be deleted to allow for cluster cleanup.
func needsDelete(service *corev1.Service) bool {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer ||
		len(service.Status.LoadBalancer.Ingress) == 0 {
		return false
	}
	return service.Status.LoadBalancer.Ingress[0].IP != "" ||
		service.Status.LoadBalancer.Ingress[0].Hostname != ""
}

func toStringSlice[T fmt.Stringer](stringers []T) []string {
	strs := make([]string, 0, len(stringers))
	for _, k := range stringers {
		strs = append(strs, k.String())
	}

	return strs
}

func failedToDeleteServicesError(svcsFailedToBeDeleted []ctrlclient.ObjectKey) error {
	return fmt.Errorf("%w: the following Services could not be deleted "+
		"and must cleaned up manually before deleting the cluster: %s",
		ErrFailedToDeleteService,
		strings.Join(toStringSlice(svcsFailedToBeDeleted), ", "),
	)
}

func servicesStillExistError(svcsStillExisting []ctrlclient.ObjectKey) error {
	return fmt.Errorf("%w: waiting for the following services to be fully deleted: %s",
		ErrServicesStillExist,
		strings.Join(toStringSlice(svcsStillExisting), ","),
	)
}

func shouldDeleteServicesWithLoadBalancer(cluster *v1beta1.Cluster) (bool, error) {
	// Use the Cluster annotations to skip deleting
	val, found := cluster.GetAnnotations()[LoadBalancerGCAnnotation]
	if !found {
		val = "true"
	}
	shouldDeleteBasedOnAnnotation, err := strconv.ParseBool(val)
	if err != nil {
		return false, fmt.Errorf(
			"failed to convert value %s of annotation %s to bool: %w",
			val,
			LoadBalancerGCAnnotation,
			err,
		)
	}

	// Use the Cluster phase to determine if it's safe to skip deleting:
	//
	// - when ClusterPhasePending or ClusterPhaseProvisioning Kubernetes API has not been created
	// and the user would not have been able to create any Kubernetes resources that would prevent cleanup
	//
	// - when ClusterPhaseDeleting it's too late to try to cleanup.
	phase := cluster.Status.GetTypedPhase()
	skipDeleteBasedOnPhase := phase == v1beta1.ClusterPhasePending ||
		phase == v1beta1.ClusterPhaseProvisioning ||
		phase == v1beta1.ClusterPhaseDeleting

	// use the Cluster conditions to determine if the API server is even reachable
	// Check v1beta1 conditions directly since we're using v1beta1 API
	controlPlaneReachable := false
	if cluster != nil {
		for _, c := range cluster.Status.Conditions {
			if c.Type == v1beta1.ControlPlaneInitializedCondition && c.Status == corev1.ConditionTrue {
				controlPlaneReachable = true
				break
			}
		}
	}

	return shouldDeleteBasedOnAnnotation && controlPlaneReachable && !skipDeleteBasedOnPhase, nil
}
