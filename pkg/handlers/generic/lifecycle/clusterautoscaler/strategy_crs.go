// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/utils"
)

type crsConfig struct {
	defaultClusterAutoscalerConfigMap string
}

func (c *crsConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultClusterAutoscalerConfigMap,
		prefix+".default-cluster-autoscaler-configmap-name",
		"cluster-autoscaler",
		"name of the ConfigMap used to deploy cluster-autoscaler",
	)
}

type crsStrategy struct {
	config crsConfig

	client ctrlclient.Client
}

func (s crsStrategy) apply(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	defaultsNamespace string,
	log logr.Logger,
) error {
	defaultCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultsNamespace,
			Name:      s.config.defaultClusterAutoscalerConfigMap,
		},
	}

	err := s.client.Get(
		ctx,
		ctrlclient.ObjectKeyFromObject(defaultCM),
		defaultCM,
	)
	if err != nil {
		return fmt.Errorf("failed to get default cluster-autoscaler ConfigMap: %w", err)
	}

	log.Info("Ensuring cluster-autoscaler ConfigMap exists for cluster")

	cluster := &req.Cluster

	data := templateData(defaultCM.Data, cluster.Name, cluster.Namespace)
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      defaultCM.Name + "-" + cluster.Name,
		},
		Data: data,
	}

	if err := client.ServerSideApply(ctx, s.client, cm, client.ForceOwnership); err != nil {
		return fmt.Errorf(
			"failed to apply cluster-autoscaler installation ConfigMap: %w",
			err,
		)
	}

	// The cluster-autoscaler is different from other addons.
	// It requires all resources to be created in the management cluster,
	// which means creating the ClusterResourceSet always targeting the management cluster.
	targetCluster, err := findTargetCluster(ctx, s.client, cluster)
	if err != nil {
		return err
	}

	// In the case when existingManagementCluster is nil, i.e. when s.client points to a bootstrap cluster,
	// it is possible that the namespace where the cluster will be moved to, and where the cluster-autoscaler resources
	// will be created, does not exist yet.
	// In that case, we need to create the namespace in the target cluster.
	clusterKey := ctrlclient.ObjectKeyFromObject(targetCluster)
	remoteClient, err := remote.NewClusterClient(ctx, "", s.client, clusterKey)
	if err != nil {
		return fmt.Errorf("error creating remote cluster client: %w", err)
	}
	if err = utils.EnsureNamespaceWithName(ctx, remoteClient, cluster.Namespace); err != nil {
		return fmt.Errorf(
			"failed to create Namespace in remote cluster: %w",
			err,
		)
	}

	if err = utils.EnsureCRSForClusterFromObjects(ctx, cm.Name, s.client, targetCluster, cm); err != nil {
		return fmt.Errorf(
			"failed to apply cluster-autoscaler installation ClusterResourceSet: %w",
			err,
		)
	}

	return nil
}
