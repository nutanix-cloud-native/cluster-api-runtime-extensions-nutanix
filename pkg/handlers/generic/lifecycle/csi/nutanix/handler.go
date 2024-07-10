// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	csiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultHelmReleaseName      = "nutanix-csi"
	defaultHelmReleaseNamespace = "ntnx-system"

	//nolint:gosec // Does not contain hard coded credentials.
	defaultCredentialsSecretName = "nutanix-csi-credentials"
)

var DefaultStorageClassParameters = map[string]string{
	"storageType":                                           "NutanixVolumes",
	"csi.storage.k8s.io/fstype":                             "xfs",
	"csi.storage.k8s.io/provisioner-secret-name":            defaultCredentialsSecretName,
	"csi.storage.k8s.io/provisioner-secret-namespace":       defaultHelmReleaseNamespace,
	"csi.storage.k8s.io/node-publish-secret-name":           defaultCredentialsSecretName,
	"csi.storage.k8s.io/node-publish-secret-namespace":      defaultHelmReleaseNamespace,
	"csi.storage.k8s.io/controller-expand-secret-name":      defaultCredentialsSecretName,
	"csi.storage.k8s.io/controller-expand-secret-namespace": defaultHelmReleaseNamespace,
}

type Config struct {
	*options.GlobalOptions

	helmAddonConfig *addons.HelmAddonConfig
}

func NewConfig(globalOptions *options.GlobalOptions) *Config {
	return &Config{
		GlobalOptions: globalOptions,
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-nutanix-csi-helm-values-template",
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
	}
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type NutanixCSI struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter
}

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *NutanixCSI {
	return &NutanixCSI{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (n *NutanixCSI) Apply(
	ctx context.Context,
	provider v1alpha1.CSIProvider,
	defaultStorage v1alpha1.DefaultStorage,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	var strategy addons.Applier
	switch provider.Strategy {
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := n.helmChartInfoGetter.For(ctx, log, config.NutanixStorageCSI)
		if err != nil {
			return fmt.Errorf(
				"failed to get configuration for Nutanix storage chart to create helm addon: %w",
				err,
			)
		}
		strategy = addons.NewHelmAddonApplier(
			n.config.helmAddonConfig,
			n.client,
			helmChart,
			config.NutanixStorageCSI,
		)
	default:
		return fmt.Errorf("strategy %s not implemented", provider.Strategy)
	}

	if provider.Credentials != nil {
		err := handlersutils.EnsureOwnerReferenceForSecret(
			ctx,
			n.client,
			provider.Credentials.SecretRef.Name,
			cluster,
		)
		if err != nil {
			return fmt.Errorf(
				"error updating owner references on Nutanix CSI driver source Secret: %w",
				err,
			)
		}
		key := ctrlclient.ObjectKey{
			Name:      defaultCredentialsSecretName,
			Namespace: defaultHelmReleaseNamespace,
		}
		err = handlersutils.CopySecretToRemoteCluster(
			ctx,
			n.client,
			provider.Credentials.SecretRef.Name,
			key,
			cluster,
		)
		if err != nil {
			return fmt.Errorf(
				"error creating credentials Secret for the Nutanix CSI driver: %w",
				err,
			)
		}
	}

	if err := strategy.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply nutanix CSI addon: %w", err)
	}

	err := csiutils.CreateStorageClassesOnRemote(
		ctx,
		n.client,
		provider.StorageClassConfigs,
		cluster,
		defaultStorage,
		v1alpha1.CSIProviderNutanix,
		v1alpha1.NutanixProvisioner,
		DefaultStorageClassParameters,
	)
	if err != nil {
		return fmt.Errorf("error creating StorageClasses for the Nutanix CSI driver: %w", err)
	}
	return nil
}
