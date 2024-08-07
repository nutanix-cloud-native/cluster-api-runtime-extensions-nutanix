// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"context"
	"crypto/md5" //nolint:gosec // Does not need to be cryptographically secure.
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
)

const (
	defaultHelmReleaseName = "cluster-autoscaler"
)

type helmAddonConfig struct {
	defaultValuesTemplateConfigMapName string
}

func (c *helmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-cluster-autoscaler-helm-values-template",
		"default values ConfigMap name",
	)
}

type helmAddonStrategy struct {
	config helmAddonConfig

	client    ctrlclient.Client
	helmChart *config.HelmChart
}

func (s helmAddonStrategy) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	defaultsNamespace string,
	log logr.Logger,
) error {
	// The cluster-autoscaler is different from other addons.
	// It requires all resources to be created in the management cluster,
	// which means creating the HelmChartProxy always targeting the management cluster.
	targetCluster, err := findTargetCluster(ctx, s.client, cluster)
	if err != nil {
		return err
	}

	applier := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			s.config.defaultValuesTemplateConfigMapName,
			cluster.Namespace,
			//nolint:gosec // Does not need to be cryptographically secure.
			fmt.Sprintf(
				"%s-%x",
				defaultHelmReleaseName,
				md5.Sum([]byte(cluster.Namespace+"/"+cluster.Name)),
			),
		),
		s.client,
		s.helmChart,
	).WithTargetCluster(targetCluster).WithValueTemplater(templateValues)

	if err = applier.Apply(ctx, cluster, defaultsNamespace, log); err != nil {
		return fmt.Errorf("failed to apply cluster-autoscaler installation HelmChartProxy: %w", err)
	}

	return nil
}

func (s helmAddonStrategy) delete(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	// The cluster-autoscaler is different from other addons.
	// It requires all resources to be created in the management cluster,
	// which means creating the HelmChartProxy always targeting the management cluster.
	targetCluster, err := findTargetCluster(ctx, s.client, cluster)
	if err != nil {
		return err
	}

	applier := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			s.config.defaultValuesTemplateConfigMapName,
			cluster.Namespace,
			//nolint:gosec // Does not need to be cryptographically secure.
			fmt.Sprintf(
				"%s-%x",
				defaultHelmReleaseName,
				md5.Sum([]byte(cluster.Namespace+"/"+cluster.Name)),
			),
		),
		s.client,
		s.helmChart,
	).WithTargetCluster(targetCluster)

	hcp, err := applier.FindExistingHelmChartProxy(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to lookup existing HelmChartProxy for cluster: %w", err)
	}

	if err := ctrlclient.IgnoreNotFound(s.client.Delete(ctx, hcp)); err != nil {
		return fmt.Errorf(
			"failed to delete cluster-autoscaler installation HelmChartProxy: %w",
			err,
		)
	}

	return nil
}
