// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

type crsConfig struct {
	defaultCiliumConfigMapName string
}

func (c *crsConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultCiliumConfigMapName,
		prefix+".default-cilium-configmap-name",
		"cilium",
		"name of the ConfigMap used to deploy Cilium",
	)
}

type crsStrategy struct {
	config crsConfig

	client ctrlclient.Client
}

func (s crsStrategy) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	defaultsNamespace string,
	log logr.Logger,
) error {
	defaultCiliumConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultsNamespace,
			Name:      s.config.defaultCiliumConfigMapName,
		},
	}

	err := s.client.Get(
		ctx,
		ctrlclient.ObjectKeyFromObject(defaultCiliumConfigMap),
		defaultCiliumConfigMap,
	)
	if err != nil {
		return fmt.Errorf("failed to get default Cilium ConfigMap: %w", err)
	}

	log.Info("Ensuring Cilium installation CRS and ConfigMap exist for cluster")

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "cilium-cni-installation-" + cluster.Name,
		},
		Data:       defaultCiliumConfigMap.Data,
		BinaryData: defaultCiliumConfigMap.BinaryData,
	}

	if err := client.ServerSideApply(ctx, s.client, cm, client.ForceOwnership); err != nil {
		return fmt.Errorf(
			"failed to apply Cilium CNI installation ConfigMap: %w",
			err,
		)
	}

	if err := utils.EnsureCRSForClusterFromObjects(
		ctx,
		cm.Name,
		s.client,
		cluster,
		utils.DefaultEnsureCRSForClusterFromObjectsOptions(),
		cm,
	); err != nil {
		return fmt.Errorf(
			"failed to apply Cilium CNI installation ClusterResourceSet: %w",
			err,
		)
	}

	return nil
}
