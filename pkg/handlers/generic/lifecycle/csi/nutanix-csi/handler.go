// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	lifecycleutils "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/utils"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	defaultHelmRepositoryURL              = "https://nutanix.github.io/helm/"
	defaultStorageHelmChartVersion        = "v2.6.6"
	defaultStorageHelmChartName           = "nutanix-csi-storage"
	defaultStorageHelmReleaseNameTemplate = "nutanix-csi-storage-%s"

	defaultSnapshotHelmChartVersion        = "v6.3.2"
	defaultSnapshotHelmChartName           = "nutanix-csi-snapshot"
	defaultSnapshotHelmReleaseNameTemplate = "nutanix-csi-snapshot-%s"
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
	if provider.Credentials != nil {
		sec := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      provider.Credentials.Name,
				Namespace: req.Cluster.Namespace,
			},
		}
		err := n.client.Get(
			ctx,
			ctrlclient.ObjectKeyFromObject(sec),
			sec,
		)
		if err != nil {
			return err
		}
		err = lifecycleutils.EnsureCRSForClusterFromObjects(
			ctx,
			fmt.Sprintf("nutanix-csi-credentials-crs-%s", req.Cluster.Name),
			n.client,
			&req.Cluster,
			sec,
		)
		if err != nil {
			return err
		}
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
	valuesTemplateConfigMap, err := lifecycleutils.RetrieveValuesTemplateConfigMap(ctx,
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
			ChartName: defaultStorageHelmChartName,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: req.Cluster.Name},
			},
			ReleaseNamespace: req.Cluster.Namespace,
			ReleaseName:      fmt.Sprintf(defaultStorageHelmReleaseNameTemplate, req.Cluster.Name),
			Version:          defaultStorageHelmChartVersion,
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

	snapshotChart := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Cluster.Namespace,
			Name:      "nutanix-csi-snapshot" + req.Cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   defaultHelmRepositoryURL,
			ChartName: defaultSnapshotHelmChartName,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: req.Cluster.Name},
			},
			ReleaseNamespace: req.Cluster.Namespace,
			ReleaseName:      fmt.Sprintf(defaultSnapshotHelmReleaseNameTemplate, req.Cluster.Name),
			Version:          defaultSnapshotHelmChartVersion,
		},
	}

	if err = controllerutil.SetOwnerReference(&req.Cluster, snapshotChart, n.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on nutanix-csi installation HelmChartProxy: %w",
			err,
		)
	}

	if err = client.ServerSideApply(ctx, n.client, snapshotChart); err != nil {
		return fmt.Errorf(
			"failed to apply nutanix-csi-snapshot installation HelmChartProxy: %w",
			err,
		)
	}

	return nil
}

func (n *NutanixCSI) createStorageClasses(ctx context.Context,
	configs []v1alpha1.StorageClassConfig,
	cluster *clusterv1.Cluster,
	defaultStorageConfig *v1alpha1.DefaultStorage,
) error {
	allStorageClasses := make([]runtime.Object, 0, len(configs))
	for _, c := range configs {
		setAsDefault := c.Name == defaultStorageConfig.StorageClassConfigName &&
			v1alpha1.CSIProviderNutanix == defaultStorageConfig.ProviderName
		allStorageClasses = append(allStorageClasses, lifecycleutils.CreateStorageClass(
			c,
			n.config.GlobalOptions.DefaultsNamespace(),
			v1alpha1.NutanixProvisioner,
			setAsDefault,
		))
	}
	cm, err := lifecycleutils.CreateConfigMapForCRS(
		fmt.Sprintf("nutanix-storageclass-cm-%s", cluster.Name),
		n.config.DefaultsNamespace(),
		allStorageClasses...,
	)
	if err != nil {
		return err
	}
	err = client.ServerSideApply(ctx, n.client, cm)
	if err != nil {
		return err
	}
	return lifecycleutils.EnsureCRSForClusterFromObjects(
		ctx,
		"nutanix-storageclass-crs",
		n.client,
		cluster,
		cm,
	)
}
