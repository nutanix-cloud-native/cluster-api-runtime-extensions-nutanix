// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallb

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	lifecycleutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	defaultHelmReleaseName      = "metallb"
	defaultHelmReleaseNamespace = "metallb-system"
)

// These labels allow the MetalLB speaker pod to obtain elevated permissions,
// which it requires in order to perform its network functionalities.
var podSecurityReleaseNamespaceLabels = map[string]string{
	"pod-security.kubernetes.io/enforce": "privileged",
	"pod-security.kubernetes.io/audit":   "privileged",
	"pod-security.kubernetes.io/warn":    "privileged",
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
	}
}

func (n *MetalLB) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Applying MetalLB installation")

	values, err := lifecycleutils.RetrieveValuesTemplate(
		ctx,
		n.client,
		n.config.defaultValuesTemplateConfigMapName,
		n.config.DefaultsNamespace(),
	)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve MetalLB installation values template ConfigMap for cluster: %w",
			err,
		)
	}

	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		n.client,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return fmt.Errorf("error creating remote cluster client: %w", err)
	}

	err = lifecycleutils.EnsureNamespaceWithMetadata(
		ctx,
		remoteClient,
		defaultHelmReleaseNamespace,
		podSecurityReleaseNamespaceLabels,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to ensure release namespace %q exists: %w",
			defaultHelmReleaseName,
			err,
		)
	}

	helmChartInfo, err := n.helmChartInfoGetter.For(ctx, log, config.MetalLB)
	if err != nil {
		return fmt.Errorf("failed to get MetalLB helm chart: %w", err)
	}

	hcp := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "metallb-" + cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   helmChartInfo.Repository,
			ChartName: helmChartInfo.Name,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
			ReleaseNamespace: defaultHelmReleaseNamespace,
			ReleaseName:      defaultHelmReleaseName,
			Version:          helmChartInfo.Version,
			ValuesTemplate:   values,
		},
	}

	lifecycleutils.SetTLSConfigForHelmChartProxyIfNeeded(hcp)
	if err = controllerutil.SetOwnerReference(cluster, hcp, n.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on MetalLB installation HelmChartProxy: %w",
			err,
		)
	}

	if err = client.ServerSideApply(ctx, n.client, hcp); err != nil {
		return fmt.Errorf("failed to apply MetalLB installation HelmChartProxy: %w", err)
	}

	return nil
}
