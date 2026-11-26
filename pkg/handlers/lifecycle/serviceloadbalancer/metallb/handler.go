// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallb

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	DefaultHelmReleaseName      = "metallb"
	DefaultHelmReleaseNamespace = "metallb-system"
)

// These labels allow the MetalLB speaker pod to obtain elevated permissions,
// which it requires in order to perform its network functionalities.
var podSecurityReleaseNamespaceLabels = map[string]string{
	"pod-security.kubernetes.io/enforce":         "privileged",
	"pod-security.kubernetes.io/enforce-version": "latest",
}

type Config struct {
	*options.GlobalOptions

	defaultValuesTemplateConfigMapName string
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-metallb-helm-values-template",
		"default values ConfigMap name",
	)
}

type MetalLB struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter
	remoteClientGetter  func(context.Context, ctrlclient.Client, ctrlclient.ObjectKey) (ctrlclient.Client, error)
}

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *MetalLB {
	return &MetalLB{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		remoteClientGetter: func(
			ctx context.Context,
			cl ctrlclient.Client,
			clusterKey ctrlclient.ObjectKey,
		) (ctrlclient.Client, error) {
			return remote.NewClusterClient(ctx, "", cl, clusterKey)
		},
	}
}

func (n *MetalLB) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Applying MetalLB installation")

	remoteClientGetter := n.remoteClientGetter
	if remoteClientGetter == nil {
		remoteClientGetter = func(
			ctx context.Context,
			cl ctrlclient.Client,
			clusterKey ctrlclient.ObjectKey,
		) (ctrlclient.Client, error) {
			return remote.NewClusterClient(ctx, "", cl, clusterKey)
		}
	}

	remoteClient, err := remoteClientGetter(
		ctx,
		n.client,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return fmt.Errorf("error creating remote cluster client: %w", err)
	}

	err = handlersutils.EnsureNamespaceWithMetadata(
		ctx,
		remoteClient,
		DefaultHelmReleaseNamespace,
		podSecurityReleaseNamespaceLabels,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to ensure release namespace %q exists: %w",
			DefaultHelmReleaseNamespace,
			err,
		)
	}

	helmChartInfo, err := n.helmChartInfoGetter.For(ctx, log, config.MetalLB)
	if err != nil {
		return fmt.Errorf("failed to get MetalLB helm chart: %w", err)
	}

	addonApplier := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			n.config.defaultValuesTemplateConfigMapName,
			DefaultHelmReleaseNamespace,
			DefaultHelmReleaseName,
		),
		n.client,
		helmChartInfo,
	)

	if err := addonApplier.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply MetalLB addon: %w", err)
	}

	return nil
}
