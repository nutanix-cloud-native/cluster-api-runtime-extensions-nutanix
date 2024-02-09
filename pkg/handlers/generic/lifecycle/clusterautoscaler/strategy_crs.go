// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	capiutils "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/utils"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/utils"
)

type crsConfig struct {
	defaultsNamespace string

	defaultClusterAutoscalerConfigMap string
}

func (c *crsConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultsNamespace,
		prefix+".defaults-namespace",
		corev1.NamespaceDefault,
		"namespace of the ConfigMap used to deploy cluster-autoscaler",
	)

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
	log logr.Logger,
) error {
	defaultCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.config.defaultsNamespace,
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

	if err := client.ServerSideApply(ctx, s.client, cm); err != nil {
		return fmt.Errorf(
			"failed to apply cluster-autoscaler installation ConfigMap: %w",
			err,
		)
	}

	existingManagementCluster, err := capiutils.ManagementCluster(ctx, s.client)
	if err != nil {
		return fmt.Errorf(
			"failed to get management Cluster: %w",
			err,
		)
	}
	// The cluster-autoscaler is different from other addons.
	// It requires all resources to be created in the management cluster,
	// which means creating the ClusterResourceSet always targeting the management cluster.
	// In most cases, target the management cluster.
	// But if existingManagementCluster is nil, i.e. when s.client points to a bootstrap cluster,
	// target the cluster and assume that will become the management cluster.
	targetCluster := existingManagementCluster
	if targetCluster == nil {
		targetCluster = cluster
	}

	if err := utils.EnsureCRSForClusterFromConfigMaps(ctx, cm.Name, s.client, targetCluster, cm); err != nil {
		return fmt.Errorf(
			"failed to apply cluster-autoscaler installation ClusterResourceSet: %w",
			err,
		)
	}

	return nil
}
