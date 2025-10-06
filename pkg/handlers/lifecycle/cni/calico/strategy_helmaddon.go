// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
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
	cluster *clusterv1.Cluster,
	defaultsNamespace string,
	log logr.Logger,
) error {
	infraKind := cluster.Spec.InfrastructureRef.Kind
	defaultInstallationConfigMapName, ok := s.config.defaultProviderInstallationValuesTemplatesConfigMapNames[infraKind]
	if !ok {
		log.Info(
			fmt.Sprintf(
				"Skipping Calico CNI handler, no default installation values ConfigMap configured for infrastructure provider %q",
				cluster.Spec.InfrastructureRef.Kind,
			),
		)
		return nil
	}

	addonApplier := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			defaultInstallationConfigMapName,
			defaultTigerOperatorNamespace,
			defaultTigeraOperatorReleaseName,
		),
		s.client,
		s.helmChart,
	)

	if err := addonApplier.Apply(ctx, cluster, defaultsNamespace, log); err != nil {
		return fmt.Errorf("failed to apply Calico addon: %w", err)
	}

	return nil
}
