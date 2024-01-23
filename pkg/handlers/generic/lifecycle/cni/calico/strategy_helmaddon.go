// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

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

	caaphv1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
)

const (
	defaultCalicoHelmRepositoryURL   = "https://docs.tigera.io/calico/charts"
	defaultCalicoHelmChartVersion    = "v3.27.0"
	defaultTigeraOperatorChartName   = "tigera-operator"
	defaultTigeraOperatorReleaseName = "tigera-operator"
	defaultTigerOperatorNamespace    = "tigera-operator"
)

type helmAddonConfig struct {
	defaultsNamespace                                        string
	defaultProviderInstallationValuesTemplatesConfigMapNames map[string]string
}

func (c *helmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultsNamespace,
		prefix+".defaultsNamespace",
		corev1.NamespaceDefault,
		"namespace of the default Helm values ConfigMaps",
	)

	flags.StringToStringVar(
		&c.defaultProviderInstallationValuesTemplatesConfigMapNames,
		prefix+".default-provider-installation-values-templates-configmap-names",
		map[string]string{
			"DockerCluster": "calico-cni-helm-values-template-dockercluster",
			"AWSCluster":    "calico-cni-helm-values-template-awscluster",
		},
		"map of provider cluster implementation type to default installation values ConfigMap name",
	)
}

type helmAddonStrategy struct {
	config helmAddonConfig

	client ctrlclient.Client
}

func (s helmAddonStrategy) apply(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
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
	valuesTemplateConfigMap, err := s.retrieveValuesTemplateConfigMap(
		ctx,
		defaultInstallationConfigMapName,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve Calico installation values template ConfigMap for cluster: %w",
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
			RepoURL:   defaultCalicoHelmRepositoryURL,
			ChartName: defaultTigeraOperatorChartName,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{capiv1.ClusterNameLabel: req.Cluster.Name},
			},
			ReleaseNamespace: defaultTigerOperatorNamespace,
			ReleaseName:      defaultTigeraOperatorReleaseName,
			Version:          defaultCalicoHelmChartVersion,
			ValuesTemplate:   valuesTemplateConfigMap.Data["values.yaml"],
		},
	}

	if err := client.ServerSideApply(ctx, s.client, hcp); err != nil {
		return fmt.Errorf("failed to apply Calico CNI installation HelmChartProxy: %w", err)
	}

	return nil
}

func (s helmAddonStrategy) retrieveValuesTemplateConfigMap(
	ctx context.Context,
	configMapName string,
) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.config.defaultsNamespace,
			Name:      configMapName,
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
