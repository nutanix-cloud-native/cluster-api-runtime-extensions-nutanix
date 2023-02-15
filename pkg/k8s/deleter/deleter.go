// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package deleter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/constants"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/k8s/annotations"
)

var ErrFailedToDeleteService = errors.New("kubernetes Services deletion failed")

type Deleter struct {
	cluster *v1beta1.Cluster
	client  client.Client
	log     logr.Logger
}

type objectMetaList []metav1.ObjectMeta

func (ol objectMetaList) asCommaSeparatedString() string {
	names := make([]string, 0, len(ol))
	for n := range ol {
		obj := ol[n]
		names = append(names, fmt.Sprintf("%s/%s", obj.Namespace, obj.Name))
	}
	return strings.Join(names, ", ")
}

func New(log logr.Logger, cluster *v1beta1.Cluster, remoteClient client.Client) Deleter {
	return Deleter{
		cluster: cluster,
		client:  remoteClient,
		log:     log,
	}
}

func (d *Deleter) DeleteServicesWithLoadBalancer(ctx context.Context) error {
	return deleteServicesWithLoadBalancer(ctx, d.client, d.log)
}

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

	var svcsFailedToBeDeleted objectMetaList
	for i := range services.Items {
		svc := &services.Items[i]
		if needsDelete(svc) {
			log.Info(fmt.Sprintf("Deleting Service %s/%s", svc.Namespace, svc.Name))
			if err = c.Delete(ctx, svc); client.IgnoreNotFound(err) != nil {
				log.Error(
					err,
					fmt.Sprintf(
						"Error deleting Service %s/%s",
						svc.Namespace,
						svc.Name,
					),
				)
				svcsFailedToBeDeleted = append(svcsFailedToBeDeleted, svc.ObjectMeta)
				continue
			}
			if err = waitForServiceDeletion(ctx, c, svc); err != nil {
				log.Error(
					err,
					fmt.Sprintf(
						"Error waiting for Service to be deleted %s/%s",
						svc.Namespace,
						svc.Name,
					),
				)
				svcsFailedToBeDeleted = append(svcsFailedToBeDeleted, svc.ObjectMeta)
			}
		}
	}
	if len(svcsFailedToBeDeleted) > 0 {
		return failedToDeleteServicesError(svcsFailedToBeDeleted)
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

func waitForServiceDeletion(
	ctx context.Context,
	c client.Client,
	service *corev1.Service,
) error {
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.5,
		Jitter:   0,
		Steps:    13,
	}
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		key := client.ObjectKey{
			Namespace: service.Namespace,
			Name:      service.Name,
		}
		err := c.Get(ctx, key, service)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
}

func failedToDeleteServicesError(svcsFailedToBeDeleted objectMetaList) error {
	return fmt.Errorf("%w: the following Services could not be deleted "+
		"and must cleaned up manually before deleting the cluster: %s",
		ErrFailedToDeleteService,
		svcsFailedToBeDeleted.asCommaSeparatedString(),
	)
}

func ShouldDeleteServicesWithLoadBalancer(cluster *v1beta1.Cluster) (bool, error) {
	// use the Cluster annotations to skip deleting
	val, found := annotations.Get(cluster, constants.LoadBalancerGCAnnotation)
	if !found {
		val = "true"
	}
	shouldDeleteBasedOnAnnotation, err := strconv.ParseBool(val)
	if err != nil {
		return false, fmt.Errorf(
			"converting value %s of annotation %s to bool: %w",
			val,
			constants.LoadBalancerGCAnnotation,
			err,
		)
	}

	// use the Cluster phase to determine if it's safe to skip deleting
	//
	// when ClusterPhasePending or ClusterPhaseProvisioning Kubernetes API has not been created
	// and the user would not have been able to create any Kubernetes resources that would prevent cleanup
	//nolint:lll // long URL cannot be split up
	// https://github.com/kubernetes-sigs/cluster-api/blob/7f879be68d15737e335b6cb39d380d1d163e06e6/controllers/cluster_controller_phases.go#L44-L50
	//
	// when ClusterPhaseDeleting it's too late to try to cleanup
	phase := cluster.Status.GetTypedPhase()
	skipDeleteBasedOnPhase := phase == v1beta1.ClusterPhasePending ||
		phase == v1beta1.ClusterPhaseProvisioning ||
		phase == v1beta1.ClusterPhaseDeleting

	// use the Cluster conditions to determine if the API server is even reachable
	controlPlaneReachable := conditions.IsTrue(cluster, v1beta1.ControlPlaneInitializedCondition)

	return shouldDeleteBasedOnAnnotation && controlPlaneReachable && !skipDeleteBasedOnPhase, nil
}
