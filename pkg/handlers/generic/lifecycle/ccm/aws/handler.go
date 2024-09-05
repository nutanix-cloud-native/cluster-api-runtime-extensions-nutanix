// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package aws

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
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	awsCCMPrefix = "aws-ccm-"

	defaultHelmReleaseNamespace = metav1.NamespaceSystem
	defaultHelmReleaseName      = "aws-cloud-controller-manager"
)

type AWSCCMConfig struct {
	*options.GlobalOptions

	kubernetesMinorVersionToAWSCCMVersion map[string]string
	helmAddonConfig                       *addons.HelmAddonConfig
}

func NewConfig(globalOptions *options.GlobalOptions) *AWSCCMConfig {
	return &AWSCCMConfig{
		GlobalOptions: globalOptions,
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-aws-ccm-helm-values-template",
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
	}
}

func (a *AWSCCMConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringToStringVar(
		&a.kubernetesMinorVersionToAWSCCMVersion,
		prefix+".aws-ccm-versions",
		nil,
		"map of minor Kubernetes version to AWS CCM version",
	)

	a.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type AWSCCM struct {
	client              ctrlclient.Client
	helmChartInfoGetter *config.HelmChartGetter
	config              *AWSCCMConfig
}

func New(
	c ctrlclient.Client,
	cfg *AWSCCMConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *AWSCCM {
	return &AWSCCM{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (a *AWSCCM) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	clusterConfig *apivariables.ClusterConfigSpec,
	log logr.Logger,
) error {
	log = log.WithValues(
		"cluster",
		ctrlclient.ObjectKeyFromObject(cluster),
	)

	if clusterConfig == nil || clusterConfig.Addons == nil ||
		clusterConfig.Addons.CCM == nil {
		log.V(5).Info("No CCM configuration found")
		return nil
	}

	var strategy addons.Applier
	switch ptr.Deref(clusterConfig.Addons.CCM.Strategy, "") {
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := a.helmChartInfoGetter.For(ctx, log, config.AWSCCM)
		if err != nil {
			return fmt.Errorf("failed to get configuration to create helm addon: %w", err)
		}
		strategy = addons.NewHelmAddonApplier(
			a.config.helmAddonConfig,
			a.client,
			helmChart,
		)
	case v1alpha1.AddonStrategyClusterResourceSet:
		strategy = crsStrategy{
			config: crsConfig{
				kubernetesMinorVersionToAWSCCMVersion: a.config.kubernetesMinorVersionToAWSCCMVersion,
			},
			client: a.client,
		}
	case "":
		return fmt.Errorf("strategy not specified for AWS CCM")
	default:
		return fmt.Errorf("strategy %s not implemented", *clusterConfig.Addons.CCM.Strategy)
	}

	if err := strategy.Apply(ctx, cluster, a.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply local-path CSI addon: %w", err)
	}

	return nil
}
