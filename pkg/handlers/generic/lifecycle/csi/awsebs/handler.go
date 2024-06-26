// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package awsebs

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	csiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	defaultHelmReleaseName      = "aws-ebs-csi-driver"
	defaultHelmReleaseNamespace = "kube-system"
)

var DefaultStorageClassParameters = map[string]string{
	"csi.storage.k8s.io/fstype": "ext4",
	"type":                      "gp3",
}

type addonStrategy interface {
	apply(
		context.Context,
		*clusterv1.Cluster,
		string,
		logr.Logger,
	) error
}

type AWSEBSConfig struct {
	*options.GlobalOptions

	crsConfig       crsConfig
	helmAddonConfig helmAddonConfig
}

func (a *AWSEBSConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	a.crsConfig.AddFlags(prefix+".crs", flags)
	a.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type AWSEBS struct {
	client              ctrlclient.Client
	helmChartInfoGetter *config.HelmChartGetter
	config              *AWSEBSConfig
}

func New(
	c ctrlclient.Client,
	cfg *AWSEBSConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *AWSEBS {
	return &AWSEBS{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (a *AWSEBS) Apply(
	ctx context.Context,
	provider v1alpha1.CSIProvider,
	defaultStorage v1alpha1.DefaultStorage,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	var strategy addonStrategy
	switch provider.Strategy {
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := a.helmChartInfoGetter.For(ctx, log, config.AWSEBSCSI)
		if err != nil {
			return fmt.Errorf("failed to get configuration to create helm addon: %w", err)
		}
		strategy = helmAddonStrategy{
			config:    a.config.helmAddonConfig,
			client:    a.client,
			helmChart: helmChart,
		}
	case v1alpha1.AddonStrategyClusterResourceSet:
		strategy = crsStrategy{
			config: a.config.crsConfig,
			client: a.client,
		}
	default:
		return fmt.Errorf("strategy %s not implemented", strategy)
	}

	if err := strategy.apply(ctx, cluster, a.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply aws-ebs CSI addon: %w", err)
	}

	err := csiutils.CreateStorageClassesOnRemote(
		ctx,
		a.client,
		provider.StorageClassConfigs,
		cluster,
		defaultStorage,
		v1alpha1.CSIProviderAWSEBS,
		v1alpha1.AWSEBSProvisioner,
		DefaultStorageClassParameters,
	)
	if err != nil {
		return fmt.Errorf("error creating StorageClasses for the AWS EBS CSI driver: %w", err)
	}
	return nil
}
