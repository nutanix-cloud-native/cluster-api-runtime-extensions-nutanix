// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultCiliumReleaseName = "cilium"
	defaultCiliumNamespace   = "kube-system"
)

type helmAddonConfig struct {
	defaultValuesTemplateConfigMapName string
}

func (c *helmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-cilium-cni-helm-values-template",
		"default values ConfigMap name",
	)
}

type helmAddonStrategy struct {
	config    helmAddonConfig
	client    ctrlclient.Client
	helmChart *config.HelmChart
}

func (s helmAddonStrategy) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	defaultsNamespace string,
	log logr.Logger,
) error {
	log.Info("Retrieving Cilium installation values template for cluster")
	values, err := handlersutils.RetrieveValuesTemplate(
		ctx,
		s.client,
		s.config.defaultValuesTemplateConfigMapName,
		defaultsNamespace,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve Cilium installation values template for cluster: %w",
			err,
		)
	}

	hcp := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "cilium-cni-installation-" + cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   s.helmChart.Repository,
			ChartName: s.helmChart.Name,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
			ReleaseNamespace: defaultCiliumNamespace,
			ReleaseName:      defaultCiliumReleaseName,
			Version:          s.helmChart.Version,
			ValuesTemplate:   values,
		},
	}

	handlersutils.SetTLSConfigForHelmChartProxyIfNeeded(hcp)
	if err := controllerutil.SetOwnerReference(cluster, hcp, s.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on Cilium CNI installation HelmChartProxy: %w",
			err,
		)
	}

	if err := client.ServerSideApply(ctx, s.client, hcp, client.ForceOwnership); err != nil {
		return fmt.Errorf("failed to apply Cilium CNI installation HelmChartProxy: %w", err)
	}

	return nil
}
