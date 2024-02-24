// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nfd

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/utils"
)

type crsConfig struct {
	defaultsNamespace string

	defaultNFDConfigMap string
}

func (c *crsConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultsNamespace,
		prefix+".defaults-namespace",
		corev1.NamespaceDefault,
		"namespace of the ConfigMap used to deploy Cilium",
	)

	flags.StringVar(
		&c.defaultNFDConfigMap,
		prefix+".default-nfd-configmap-name",
		"node-feature-discovery",
		"name of the ConfigMap used to deploy Node Feature Discovery (NFD)",
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
			Name:      s.config.defaultNFDConfigMap,
		},
	}

	err := s.client.Get(
		ctx,
		ctrlclient.ObjectKeyFromObject(defaultCM),
		defaultCM,
	)
	if err != nil {
		return fmt.Errorf("failed to get default NFD ConfigMap: %w", err)
	}

	log.Info("Ensuring NFD ConfigMap exists for cluster")

	cluster := &req.Cluster

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      defaultCM.Name + "-" + cluster.Name,
		},
		Data:       defaultCM.Data,
		BinaryData: defaultCM.BinaryData,
	}

	if err := client.ServerSideApply(ctx, s.client, cm); err != nil {
		return fmt.Errorf(
			"failed to apply NFD installation ConfigMap: %w",
			err,
		)
	}

	if err := utils.EnsureCRSForClusterFromConfigMaps(ctx, cm.Name, s.client, cluster, cm); err != nil {
		return fmt.Errorf(
			"failed to apply NFD installation ClusterResourceSet: %w",
			err,
		)
	}

	return nil
}
