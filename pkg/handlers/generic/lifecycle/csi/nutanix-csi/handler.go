// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/utils"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	defaultHelmRepositoryURL       = "https://nutanix.github.io/helm/"
	defaultHelmChartVersion        = "v2.6.6"
	defaultHelmChartName           = "nutanix-csi-storage"
	defaultHelmReleaseNameTemplate = "nutanix-csi-storage-%s"
	nutanixCSIProvisionerName      = "csi.nutanix.com"
)

type NutanixCSIConfig struct {
	*options.GlobalOptions
	defaultValuesTemplateConfigMapName string
}

func (n *NutanixCSIConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&n.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-nutanix-csi-helm-values-template",
		"default values ConfigMap name",
	)
}

type NutanixCSI struct {
	client ctrlclient.Client
	config *NutanixCSIConfig
}

func New(
	c ctrlclient.Client,
	cfg *NutanixCSIConfig,
) *NutanixCSI {
	return &NutanixCSI{
		client: c,
		config: cfg,
	}
}

func (n *NutanixCSI) Apply(
	ctx context.Context,
	provider v1alpha1.CSIProvider,
	defaultStorageConfig *v1alpha1.DefaultStorage,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
) error {
	strategy := provider.Strategy
	switch strategy {
	case v1alpha1.AddonStrategyHelmAddon:
		err := n.handleHelmAddonApply(ctx, req)
		if err != nil {
			return err
		}
	case v1alpha1.AddonStrategyClusterResourceSet:
	default:
		return fmt.Errorf("stategy %s not implemented", strategy)
	}
	return n.createStorageClasses(
		ctx,
		provider.StorageClassConfig,
		&req.Cluster,
		defaultStorageConfig,
	)
}

func (n *NutanixCSI) handleHelmAddonApply(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
) error {
	valuesTemplateConfigMap, err := utils.RetrieveValuesTemplateConfigMap(ctx,
		n.client,
		n.config.defaultValuesTemplateConfigMapName,
		n.config.DefaultsNamespace())
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve nutanix csi installation values template ConfigMap for cluster: %w",
			err,
		)
	}
	values := valuesTemplateConfigMap.Data["values.yaml"]

	hcp := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Cluster.Namespace,
			Name:      "nutanix-csi-" + req.Cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   defaultHelmRepositoryURL,
			ChartName: defaultHelmChartName,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: req.Cluster.Name},
			},
			ReleaseNamespace: req.Cluster.Namespace,
			ReleaseName:      fmt.Sprintf(defaultHelmReleaseNameTemplate, req.Cluster.Name),
			Version:          defaultHelmChartVersion,
			ValuesTemplate:   values,
		},
	}

	if err = controllerutil.SetOwnerReference(&req.Cluster, hcp, n.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on nutanix-csi installation HelmChartProxy: %w",
			err,
		)
	}

	if err = client.ServerSideApply(ctx, n.client, hcp); err != nil {
		return fmt.Errorf("failed to apply nutanix-csi installation HelmChartProxy: %w", err)
	}

	return nil
}

func (n *NutanixCSI) createStorageClasses(ctx context.Context,
	configs []v1alpha1.StorageClassConfig,
	cluster *clusterv1.Cluster,
	defaultStorageConfig *v1alpha1.DefaultStorage,
) error {
	for _, c := range configs {
		setAsDefault := c.Name == defaultStorageConfig.StorageClassConfigName &&
			v1alpha1.CSIProviderNutanix == defaultStorageConfig.ProviderName
		err := utils.CreateStorageClass(
			ctx,
			n.client,
			c,
			cluster,
			nutanixCSIProvisionerName,
			n.config.GlobalOptions.DefaultsNamespace(),
			setAsDefault,
		)
		if err != nil {
			return fmt.Errorf("failed to create storageclass %w", err)
		}
	}
	return nil
}
