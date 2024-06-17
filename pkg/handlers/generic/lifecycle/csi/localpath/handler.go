// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package localpath

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultHelmReleaseName      = "local-path-provisioner"
	defaultHelmReleaseNamespace = "kube-system"
)

type LocalPathProvisionerCSI struct {
	client              ctrlclient.Client
	helmChartInfoGetter *config.HelmChartGetter
}

func New(
	c ctrlclient.Client,
	helmChartInfoGetter *config.HelmChartGetter,
) *LocalPathProvisionerCSI {
	return &LocalPathProvisionerCSI{
		client:              c,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (l *LocalPathProvisionerCSI) Apply(
	ctx context.Context,
	provider v1alpha1.CSIProvider,
	defaultStorageConfig v1alpha1.DefaultStorage,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	log logr.Logger,
) error {
	strategy := provider.Strategy
	switch strategy {
	case v1alpha1.AddonStrategyHelmAddon:
		err := l.handleHelmAddonApply(ctx, req, log)
		if err != nil {
			return err
		}
	case v1alpha1.AddonStrategyClusterResourceSet:
	default:
		return fmt.Errorf("strategy %s not implemented", strategy)
	}

	err := handlersutils.CreateStorageClassOnRemote(
		ctx,
		l.client,
		provider.StorageClassConfig,
		&req.Cluster,
		defaultStorageConfig,
		v1alpha1.CSIProviderLocalPath,
		v1alpha1.LocalPathProvisioner,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"error creating StorageClasses for the local-path-provisioner CSI driver: %w",
			err,
		)
	}
	return nil
}

func (l *LocalPathProvisionerCSI) handleHelmAddonApply(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	log logr.Logger,
) error {
	chart, err := l.helmChartInfoGetter.For(ctx, log, config.LocalPathProvisionerCSI)
	if err != nil {
		return fmt.Errorf("failed to get helm chart %q: %w", config.LocalPathProvisionerCSI, err)
	}

	valuesTemplate := `
storageClass:
  create: false
  provisionerName: rancher.io/local-path
helperImage:
  tag: 1.36.1
`

	chartProxy := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Cluster.Namespace,
			Name:      "local-path-provisioner-csi-" + req.Cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   chart.Repository,
			ChartName: chart.Name,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: req.Cluster.Name},
			},
			ReleaseNamespace: defaultHelmReleaseNamespace,
			ReleaseName:      defaultHelmReleaseName,
			Version:          chart.Version,
			ValuesTemplate:   valuesTemplate,
		},
	}
	handlersutils.SetTLSConfigForHelmChartProxyIfNeeded(chartProxy)
	if err = controllerutil.SetOwnerReference(&req.Cluster, chartProxy, l.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on HelmChartProxy %q: %w",
			chartProxy.Name,
			err,
		)
	}

	if err = client.ServerSideApply(ctx, l.client, chartProxy, client.ForceOwnership); err != nil {
		return fmt.Errorf("failed to apply HelmChartProxy %q: %w", chartProxy.Name, err)
	}

	return nil
}
