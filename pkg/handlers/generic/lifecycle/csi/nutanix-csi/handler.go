// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	csiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultStorageHelmReleaseName      = "nutanix-csi-storage"
	defaultStorageHelmReleaseNamespace = "ntnx-system"

	defaultSnapshotHelmReleaseName      = "nutanix-csi-snapshot"
	defaultSnapshotHelmReleaseNamespace = "ntnx-system"

	//nolint:gosec // Does not contain hard coded credentials.
	defaultCredentialsSecretName = "nutanix-csi-credentials"
)

var defaultStorageClassParameters = map[string]string{
	"storageType":                                           "NutanixVolumes",
	"csi.storage.k8s.io/fstype":                             "xfs",
	"csi.storage.k8s.io/provisioner-secret-name":            defaultCredentialsSecretName,
	"csi.storage.k8s.io/provisioner-secret-namespace":       defaultStorageHelmReleaseNamespace,
	"csi.storage.k8s.io/node-publish-secret-name":           defaultCredentialsSecretName,
	"csi.storage.k8s.io/node-publish-secret-namespace":      defaultStorageHelmReleaseNamespace,
	"csi.storage.k8s.io/controller-expand-secret-name":      defaultCredentialsSecretName,
	"csi.storage.k8s.io/controller-expand-secret-namespace": defaultStorageHelmReleaseNamespace,
}

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
	client              ctrlclient.Client
	config              *NutanixCSIConfig
	helmChartInfoGetter *config.HelmChartGetter
}

func New(
	c ctrlclient.Client,
	cfg *NutanixCSIConfig,
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
	strategy := provider.Strategy
	switch strategy {
	case v1alpha1.AddonStrategyHelmAddon:
		err := n.handleHelmAddonApply(ctx, cluster, log)
		if err != nil {
			return err
		}
	case v1alpha1.AddonStrategyClusterResourceSet:
	default:
		return fmt.Errorf("stategy %s not implemented", strategy)
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
			Namespace: defaultStorageHelmReleaseNamespace,
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

	err := csiutils.CreateStorageClassOnRemote(
		ctx,
		n.client,
		provider.StorageClassConfigs,
		cluster,
		defaultStorage,
		v1alpha1.CSIProviderNutanix,
		v1alpha1.NutanixProvisioner,
		defaultStorageClassParameters,
	)
	if err != nil {
		return fmt.Errorf("error creating StorageClasses for the Nutanix CSI driver: %w", err)
	}
	return nil
}

func (n *NutanixCSI) handleHelmAddonApply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Retrieving Nutanix CSI installation values template for cluster")
	values, err := handlersutils.RetrieveValuesTemplate(
		ctx,
		n.client,
		n.config.defaultValuesTemplateConfigMapName,
		n.config.DefaultsNamespace(),
	)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve nutanix csi installation values template ConfigMap for cluster: %w",
			err,
		)
	}

	storageChart, err := n.helmChartInfoGetter.For(ctx, log, config.NutanixStorageCSI)
	if err != nil {
		return fmt.Errorf("failed to get helm chart %q: %w", config.NutanixStorageCSI, err)
	}

	snapshotChart, err := n.helmChartInfoGetter.For(ctx, log, config.NutanixSnapshotCSI)
	if err != nil {
		return fmt.Errorf("failed to get helm chart %q: %w", config.NutanixSnapshotCSI, err)
	}

	storageChartProxy := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "nutanix-csi-" + cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   storageChart.Repository,
			ChartName: storageChart.Name,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
			ReleaseNamespace: defaultStorageHelmReleaseNamespace,
			ReleaseName:      defaultStorageHelmReleaseName,
			Version:          storageChart.Version,
			ValuesTemplate:   values,
		},
	}
	handlersutils.SetTLSConfigForHelmChartProxyIfNeeded(storageChartProxy)
	snapshotChartProxy := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "nutanix-csi-snapshot-" + cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   snapshotChart.Repository,
			ChartName: snapshotChart.Name,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
			ReleaseNamespace: defaultSnapshotHelmReleaseNamespace,
			ReleaseName:      defaultSnapshotHelmReleaseName,
			Version:          snapshotChart.Version,
		},
	}
	handlersutils.SetTLSConfigForHelmChartProxyIfNeeded(snapshotChartProxy)
	// We use a slice of pointers to satisfy the gocritic linter rangeValCopy check.
	for _, cp := range []*caaphv1.HelmChartProxy{storageChartProxy, snapshotChartProxy} {
		if err = controllerutil.SetOwnerReference(cluster, cp, n.client.Scheme()); err != nil {
			return fmt.Errorf(
				"failed to set owner reference on HelmChartProxy %q: %w",
				cp.Name,
				err,
			)
		}

		if err = client.ServerSideApply(ctx, n.client, cp, client.ForceOwnership); err != nil {
			return fmt.Errorf("failed to apply HelmChartProxy %q: %w", cp.Name, err)
		}
	}

	return nil
}
