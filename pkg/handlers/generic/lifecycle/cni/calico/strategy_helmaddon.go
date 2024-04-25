// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	lifecycleutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/utils"
)

const (
	defaultTigeraOperatorReleaseName = "tigera-operator"
	defaultTigerOperatorNamespace    = "tigera-operator"
)

type helmAddonConfig struct {
	defaultProviderInstallationValuesTemplatesConfigMapNames map[string]string
}

func (c *helmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringToStringVar(
		&c.defaultProviderInstallationValuesTemplatesConfigMapNames,
		prefix+".default-provider-installation-values-templates-configmap-names",
		map[string]string{
			"DockerCluster":  "calico-cni-helm-values-template-dockercluster",
			"AWSCluster":     "calico-cni-helm-values-template-awscluster",
			"NutanixCluster": "calico-cni-helm-values-template-nutanixcluster",
		},
		"map of provider cluster implementation type to default installation values ConfigMap name",
	)
}

type helmAddonStrategy struct {
	config    helmAddonConfig
	helmChart *config.HelmChart
	client    ctrlclient.Client
}

func (s helmAddonStrategy) apply(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	defaultsNamespace string,
	log logr.Logger,
) error {
	infraKind := req.Cluster.Spec.InfrastructureRef.Kind
	defaultInstallationConfigMapName, ok := s.config.defaultProviderInstallationValuesTemplatesConfigMapNames[infraKind]
	if !ok {
		log.Info(
			fmt.Sprintf(
				"Skipping Calico CNI handler, no default installation values ConfigMap configured for infrastructure provider %q",
				req.Cluster.Spec.InfrastructureRef.Kind,
			),
		)
		return nil
	}

	log.Info("Retrieving Calico installation values template for cluster")
	values, err := lifecycleutils.RetrieveValuesTemplate(
		ctx,
		s.client,
		defaultInstallationConfigMapName,
		defaultsNamespace,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve Calico installation values template for cluster: %w",
			err,
		)
	}

	hcp := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Cluster.Namespace,
			Name:      "calico-cni-installation-" + req.Cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   s.helmChart.Repository,
			ChartName: s.helmChart.Name,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: req.Cluster.Name},
			},
			ReleaseNamespace: defaultTigerOperatorNamespace,
			ReleaseName:      defaultTigeraOperatorReleaseName,
			Version:          s.helmChart.Version,
			ValuesTemplate:   values,
		},
	}

	if err := controllerutil.SetOwnerReference(&req.Cluster, hcp, s.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on Calico CNI installation HelmChartProxy: %w",
			err,
		)
	}

	if err := client.ServerSideApply(ctx, s.client, hcp, client.ForceOwnership); err != nil {
		return fmt.Errorf("failed to apply Calico CNI installation HelmChartProxy: %w", err)
	}

	return nil
}
