// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nfd

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
)

const (
	defaultHelmReleaseName      = "node-feature-discovery"
	defaultHelmReleaseNamespace = "node-feature-discovery"
)

type helmAddonConfig struct {
	defaultValuesTemplateConfigMapName string
}

func (c *helmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-nfd-helm-values-template",
		"default values ConfigMap name",
	)
}

type helmAddonStrategy struct {
	config helmAddonConfig

	client    ctrlclient.Client
	helmChart *config.HelmChart
}

func (s helmAddonStrategy) apply(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	defaultsNamespace string,
	log logr.Logger,
) error {
	log.Info("Retrieving NFD installation values template for cluster")
	valuesTemplateConfigMap, err := s.retrieveValuesTemplateConfigMap(ctx, defaultsNamespace)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve NFD installation values template ConfigMap for cluster: %w",
			err,
		)
	}

	values := valuesTemplateConfigMap.Data["values.yaml"]
	values += fmt.Sprintf(`
image:
  tag: v%s-minimal
`, s.helmChart.Version)

	hcp := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Cluster.Namespace,
			Name:      "node-feature-discovery-" + req.Cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   s.helmChart.Repository,
			ChartName: s.helmChart.Name,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{capiv1.ClusterNameLabel: req.Cluster.Name},
			},
			ReleaseNamespace: defaultHelmReleaseNamespace,
			ReleaseName:      defaultHelmReleaseName,
			Version:          s.helmChart.Version,
			ValuesTemplate:   values,
		},
	}

	if err := controllerutil.SetOwnerReference(&req.Cluster, hcp, s.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on NFD installation HelmChartProxy: %w",
			err,
		)
	}

	if err := client.ServerSideApply(ctx, s.client, hcp); err != nil {
		return fmt.Errorf("failed to apply NFD installation HelmChartProxy: %w", err)
	}

	return nil
}

func (s helmAddonStrategy) retrieveValuesTemplateConfigMap(
	ctx context.Context,
	defaultsNamespace string,
) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultsNamespace,
			Name:      s.config.defaultValuesTemplateConfigMapName,
		},
	}
	configMapObjName := ctrlclient.ObjectKeyFromObject(
		configMap,
	)
	err := s.client.Get(ctx, configMapObjName, configMap)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to retrieve installation values template ConfigMap %q: %w",
			configMapObjName,
			err,
		)
	}

	return configMap, nil
}
