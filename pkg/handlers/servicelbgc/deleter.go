// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Copyright 2023 D2iQ, Inc. All rights reserved.
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
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LoadBalancerGCAnnotation = "capiext.labs.d2iq.io/loadbalancer-gc"
)

var (
	ErrFailedToDeleteService = errors.New("failed to delete kubernetes services")
	ErrServicesStillExist    = errors.New("waiting for kubernetes services to be fully deleted")
)

func deleteServicesWithLoadBalancer(
	ctx context.Context,
	c client.Client,
	log logr.Logger,
) error {
	log.Info("Listing Services with type LoadBalancer")
	services := &corev1.ServiceList{}
	err := c.List(ctx, services)
	if err != nil {
		return fmt.Errorf("error listing Services: %w", err)
	}

	var (
		svcsFailedToBeDeleted []client.ObjectKey
		svcsStillExisting     []client.ObjectKey
	)
	for idx := range services.Items {
		svc := &services.Items[idx]
		svcKey := client.ObjectKeyFromObject(svc)
		if needsDelete(svc) {
			svcsStillExisting = append(svcsStillExisting, svcKey)

			if svc.DeletionTimestamp != nil {
				continue
			}

			log.Info(fmt.Sprintf("Deleting Service %s", svcKey))
			if err = c.Delete(ctx, svc); err != nil {
				if client.IgnoreNotFound(err) == nil {
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

func failedToDeleteServicesError(svcsFailedToBeDeleted []client.ObjectKey) error {
	return fmt.Errorf("%w: the following Services could not be deleted "+
		"and must cleaned up manually before deleting the cluster: %s",
		ErrFailedToDeleteService,
		strings.Join(toStringSlice(svcsFailedToBeDeleted), ","),
	)
}

func servicesStillExistError(svcsStillExisting []client.ObjectKey) error {
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
	controlPlaneReachable := conditions.IsTrue(cluster, v1beta1.ControlPlaneInitializedCondition)

	return shouldDeleteBasedOnAnnotation && controlPlaneReachable && !skipDeleteBasedOnPhase, nil
}
