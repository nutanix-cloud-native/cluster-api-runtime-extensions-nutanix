// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/utils"
)

const (
	defaultHelmReleaseNameTemplate = "cluster-autoscaler-%s"
)

type helmAddonConfig struct {
	defaultValuesTemplateConfigMapName string
}

func (c *helmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-cluster-autoscaler-helm-values-template",
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
	log.Info("Retrieving cluster-autoscaler installation values template for cluster")
	valuesTemplateConfigMap, err := utils.RetrieveValuesTemplateConfigMap(
		ctx,
		s.client,
		s.config.defaultValuesTemplateConfigMapName,
		defaultsNamespace,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve cluster-autoscaler installation values template ConfigMap for cluster: %w",
			err,
		)
	}

	cluster := &req.Cluster

	values := valuesTemplateConfigMap.Data["values.yaml"]

	// The cluster-autoscaler is different from other addons.
	// It requires all resources to be created in the management cluster,
	// which means creating the HelmChartProxy always targeting the management cluster.
	targetCluster, err := findTargetCluster(ctx, s.client, cluster)
	if err != nil {
		return err
	}

	hcp := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: targetCluster.Namespace,
			Name:      "cluster-autoscaler-" + req.Cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   s.helmChart.Repository,
			ChartName: s.helmChart.Name,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{capiv1.ClusterNameLabel: targetCluster.Name},
			},
			ReleaseNamespace: req.Cluster.Namespace,
			ReleaseName:      fmt.Sprintf(defaultHelmReleaseNameTemplate, req.Cluster.Name),
			Version:          s.helmChart.Version,
			ValuesTemplate:   values,
		},
	}

	if err = controllerutil.SetOwnerReference(&req.Cluster, hcp, s.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on cluster-autoscaler installation HelmChartProxy: %w",
			err,
		)
	}

	if err = client.ServerSideApply(ctx, s.client, hcp); err != nil {
		return fmt.Errorf("failed to apply cluster-autoscaler installation HelmChartProxy: %w", err)
	}

	return nil
}
