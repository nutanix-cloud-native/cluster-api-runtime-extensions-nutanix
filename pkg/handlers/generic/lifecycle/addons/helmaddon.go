// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	k8sclient "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	lifecycleconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

type HelmAddonConfig struct {
	defaultValuesTemplateConfigMapName string

	defaultHelmReleaseNamespace string
	defaultHelmReleaseName      string
}

func NewHelmAddonConfig(
	defaultValuesTemplateConfigMapName string,
	defaultHelmReleaseNamespace string,
	defaultHelmReleaseName string,
) *HelmAddonConfig {
	return &HelmAddonConfig{
		defaultValuesTemplateConfigMapName: defaultValuesTemplateConfigMapName,
		defaultHelmReleaseNamespace:        defaultHelmReleaseNamespace,
		defaultHelmReleaseName:             defaultHelmReleaseName,
	}
}

func (c *HelmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		c.defaultValuesTemplateConfigMapName,
		"default values ConfigMap name",
	)
}

type helmAddonApplier struct {
	config    *HelmAddonConfig
	client    ctrlclient.Client
	chartName config.Component
	helmChart *lifecycleconfig.HelmChart
}

var _ Applier = &helmAddonApplier{}

func NewHelmAddonApplier(
	config *HelmAddonConfig,
	client ctrlclient.Client,
	helmChart *lifecycleconfig.HelmChart,
	name config.Component,
) *helmAddonApplier {
	return &helmAddonApplier{
		config:    config,
		client:    client,
		helmChart: helmChart,
		chartName: name,
	}
}

func (a *helmAddonApplier) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	defaultsNamespace string,
	log logr.Logger,
) error {
	log.Info("Retrieving installation values template for cluster")
	values, err := handlersutils.RetrieveValuesTemplate(
		ctx,
		a.client,
		a.config.defaultValuesTemplateConfigMapName,
		defaultsNamespace,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve installation values template for cluster: %w",
			err,
		)
	}

	chartProxy := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      a.config.defaultHelmReleaseName + "-" + cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   a.helmChart.Repository,
			ChartName: string(a.chartName),
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
			ReleaseNamespace: a.config.defaultHelmReleaseNamespace,
			ReleaseName:      a.config.defaultHelmReleaseName,
			Version:          a.helmChart.Version,
			ValuesTemplate:   values,
		},
	}
	handlersutils.SetTLSConfigForHelmChartProxyIfNeeded(chartProxy)
	if err = controllerutil.SetOwnerReference(cluster, chartProxy, a.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on HelmChartProxy %q: %w",
			chartProxy.Name,
			err,
		)
	}

	if err = k8sclient.ServerSideApply(ctx, a.client, chartProxy, k8sclient.ForceOwnership); err != nil {
		return fmt.Errorf("failed to apply HelmChartProxy %q: %w", chartProxy.Name, err)
	}

	return nil
}
