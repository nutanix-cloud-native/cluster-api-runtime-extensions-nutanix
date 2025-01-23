// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package localpath

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	csiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	defaultHelmReleaseName      = "local-path-provisioner-csi"
	defaultHelmReleaseNamespace = metav1.NamespaceSystem
)

type Config struct {
	*options.GlobalOptions

	crsConfig       crsConfig
	helmAddonConfig *addons.HelmAddonConfig
}

func NewConfig(globalOptions *options.GlobalOptions) *Config {
	return &Config{
		GlobalOptions: globalOptions,
		crsConfig:     crsConfig{},
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-local-path-provisioner-csi-helm-values-template",
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
	}
}

type LocalPathProvisionerCSI struct {
	client              ctrlclient.Client
	helmChartInfoGetter *config.HelmChartGetter
	config              *Config
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.crsConfig.AddFlags(prefix+".crs", flags)
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *LocalPathProvisionerCSI {
	return &LocalPathProvisionerCSI{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (l *LocalPathProvisionerCSI) Apply(
	ctx context.Context,
	provider v1alpha1.CSIProvider,
	defaultStorage v1alpha1.DefaultStorage,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	var strategy addons.Applier
	switch ptr.Deref(provider.Strategy, "") {
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := l.helmChartInfoGetter.For(
			ctx,
			log,
			cluster,
			config.LocalPathProvisionerCSI,
		)
		if err != nil {
			return fmt.Errorf("failed to get configuration to create helm addon: %w", err)
		}
		strategy = addons.NewHelmAddonApplier(
			l.config.helmAddonConfig,
			l.client,
			helmChart,
		)
	case v1alpha1.AddonStrategyClusterResourceSet:
		strategy = crsStrategy{
			config: l.config.crsConfig,
			client: l.client,
		}
	case "":
		return fmt.Errorf("strategy not provided for local-path CSI addon")
	default:
		return fmt.Errorf("strategy %s not implemented", *provider.Strategy)
	}

	if err := strategy.Apply(ctx, cluster, l.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply local-path CSI addon: %w", err)
	}

	err := csiutils.CreateStorageClassesOnRemote(
		ctx,
		l.client,
		provider.StorageClassConfigs,
		cluster,
		defaultStorage,
		v1alpha1.CSIProviderLocalPath,
		v1alpha1.LocalPathProvisioner,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"error creating StorageClasses for the local-path CSI driver: %w",
			err,
		)
	}
	return nil
}
