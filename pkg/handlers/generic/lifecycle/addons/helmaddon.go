// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	k8sclient "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	lifecycleconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

var (
	HelmReleaseNameHashLabel = "addons.cluster.x-k8s.io/helm-release-name-hash"
	ClusterNamespaceLabel    = clusterv1.ClusterNamespaceAnnotation
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
	helmChart *lifecycleconfig.HelmChart
	opts      []applyOption
}

var _ Applier = &helmAddonApplier{}

func NewHelmAddonApplier(
	config *HelmAddonConfig,
	client ctrlclient.Client,
	helmChart *lifecycleconfig.HelmChart,
) *helmAddonApplier {
	return &helmAddonApplier{
		config:    config,
		client:    client,
		helmChart: helmChart,
	}
}

type applyOptions struct {
	valueTemplater func(cluster *clusterv1.Cluster, valuesTemplate string) (string, error)
	targetCluster  *clusterv1.Cluster
}

type applyOption func(*applyOptions)

func (a *helmAddonApplier) WithValueTemplater(
	valueTemplater func(cluster *clusterv1.Cluster, valuesTemplate string) (string, error),
) *helmAddonApplier {
	a.opts = append(a.opts, func(o *applyOptions) {
		o.valueTemplater = valueTemplater
	})

	return a
}

func (a *helmAddonApplier) WithTargetCluster(cluster *clusterv1.Cluster) *helmAddonApplier {
	a.opts = append(a.opts, func(o *applyOptions) {
		o.targetCluster = cluster
	})

	return a
}

func (a *helmAddonApplier) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	defaultsNamespace string,
	log logr.Logger,
) error {
	applyOpts := &applyOptions{}
	for _, opt := range a.opts {
		opt(applyOpts)
	}

	log.Info("Checking for existing HelmChartProxy for cluster")
	chartProxy, err := a.FindExistingHelmChartProxy(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to lookup existing HelmChartProxy for cluster: %w", err)
	}

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

	if applyOpts.valueTemplater != nil {
		values, err = applyOpts.valueTemplater(cluster, values)
		if err != nil {
			return fmt.Errorf("failed to template Helm values: %w", err)
		}
	}

	targetCluster := cluster
	if applyOpts.targetCluster != nil {
		targetCluster = applyOpts.targetCluster
	}

	if chartProxy == nil {
		chartProxy = &caaphv1.HelmChartProxy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: caaphv1.GroupVersion.String(),
				Kind:       "HelmChartProxy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: targetCluster.Namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: cluster.Name,
					ClusterNamespaceLabel:      cluster.Namespace,
					// Label values have a maximum length of 63 characters so hash the release name to
					// ensure it fits within the limit.
					HelmReleaseNameHashLabel: hashReleaseName(a.config.defaultHelmReleaseName),
				},
				GenerateName: fmt.Sprintf("%s-", a.config.defaultHelmReleaseName),
			},
		}
	}

	chartProxy.Spec = caaphv1.HelmChartProxySpec{
		RepoURL:   a.helmChart.Repository,
		ChartName: a.helmChart.Name,
		ClusterSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{clusterv1.ClusterNameLabel: targetCluster.Name},
		},
		ReleaseNamespace: a.config.defaultHelmReleaseNamespace,
		ReleaseName:      a.config.defaultHelmReleaseName,
		Version:          a.helmChart.Version,
		ValuesTemplate:   values,
	}

	handlersutils.SetTLSConfigForHelmChartProxyIfNeeded(chartProxy)
	if err = controllerutil.SetOwnerReference(targetCluster, chartProxy, a.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on HelmChartProxy %q: %w",
			chartProxy.Name,
			err,
		)
	}

	// Only server-side apply the HelmChartProxy if it already existed, i.e. that metadata.name is non-empty.
	// This allows to use metadata.generateName for the first creation to avoid naming collisions.
	if chartProxy.Name == "" {
		if err = a.client.Create(ctx, chartProxy); err != nil {
			return fmt.Errorf("failed to create HelmChartProxy %q: %w", chartProxy.Name, err)
		}
	} else {
		// metadata.managedFields must be nil when using server-side apply.
		chartProxy.ManagedFields = nil
		if err = k8sclient.ServerSideApply(ctx, a.client, chartProxy, k8sclient.ForceOwnership); err != nil {
			return fmt.Errorf("failed to apply HelmChartProxy %q: %w", chartProxy.Name, err)
		}
	}

	return nil
}

func (a *helmAddonApplier) FindExistingHelmChartProxy(
	ctx context.Context, cluster *clusterv1.Cluster,
) (*caaphv1.HelmChartProxy, error) {
	applyOpts := &applyOptions{}
	for _, opt := range a.opts {
		opt(applyOpts)
	}

	targetCluster := cluster
	if applyOpts.targetCluster != nil {
		targetCluster = applyOpts.targetCluster
	}

	chartProxyList := &caaphv1.HelmChartProxyList{}
	if err := a.client.List(
		ctx,
		chartProxyList,
		ctrlclient.MatchingLabels{
			clusterv1.ClusterNameLabel: cluster.Name,
			ClusterNamespaceLabel:      cluster.Namespace,
			HelmReleaseNameHashLabel:   hashReleaseName(a.config.defaultHelmReleaseName),
		},
		ctrlclient.InNamespace(targetCluster.Namespace),
	); err != nil {
		return nil, fmt.Errorf("failed to list HelmChartProxies: %w", err)
	}

	if len(chartProxyList.Items) == 0 {
		return nil, nil
	}

	if len(chartProxyList.Items) > 1 {
		return nil, fmt.Errorf("found multiple HelmChartProxies for cluster %q", cluster.Name)
	}

	return &chartProxyList.Items[0], nil
}

func hashReleaseName(releaseName string) string {
	// Use Sum224 to ensure the hash is 56 characters long.
	return fmt.Sprintf("%x", sha256.Sum224([]byte(releaseName)))
}
