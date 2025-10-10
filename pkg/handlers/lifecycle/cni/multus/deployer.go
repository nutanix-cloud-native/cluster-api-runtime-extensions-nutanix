// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
)

const (
	defaultMultusReleaseName = "multus"
	defaultMultusNamespace   = metav1.NamespaceSystem
)

type MultusDeployer struct {
	client              ctrlclient.Client
	helmChartInfoGetter *config.HelmChartGetter
}

func NewMultusDeployer(client ctrlclient.Client, helmChartInfoGetter *config.HelmChartGetter) *MultusDeployer {
	return &MultusDeployer{
		client:              client,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (m *MultusDeployer) Deploy(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	readinessSocketPath string,
	targetNamespace string,
	log logr.Logger,
) error {
	// Check if Multus deployment is supported for this cloud provider
	isSupported, providerName := m.isCloudProviderSupported(cluster)

	// Currently, Multus is only supported for EKS and Nutanix clusters
	if !isSupported {
		log.Info("Multus deployment is only supported for EKS and Nutanix clusters. Skipping deployment.")
		return nil
	}

	log.Info(fmt.Sprintf("Cluster is %s. Proceeding with Multus deployment.", providerName))

	// Get Multus Helm chart info
	helmChart, err := m.helmChartInfoGetter.For(ctx, log, config.Multus)
	if err != nil {
		// For local testing, provide a placeholder chart info
		log.Info("Multus not found in helm-config.yaml, using placeholder chart info for local development")
		helmChart = &config.HelmChart{
			Name:       "multus",
			Version:    "dev",
			Repository: "",
		}
	}

	// Deploy Multus using HelmAddonApplier with template function
	strategy := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			"default-multus-values-template",
			defaultMultusNamespace,
			defaultMultusReleaseName,
		),
		m.client,
		helmChart,
	).WithValueTemplater(templateValuesFunc(readinessSocketPath)).
		WithDefaultWaiter()

	if err := strategy.Apply(ctx, cluster, targetNamespace, log); err != nil {
		return fmt.Errorf("failed to apply Multus deployment: %w", err)
	}

	log.Info("Successfully deployed Multus")
	return nil
}

// isCloudProviderSupported checks if the cluster is a supported cloud provider
// by inspecting the infrastructure reference.
func (m *MultusDeployer) isCloudProviderSupported(cluster *clusterv1.Cluster) (
	isSupported bool,
	providerName string,
) {
	if cluster.Spec.InfrastructureRef == nil {
		return false, ""
	}

	switch cluster.Spec.InfrastructureRef.Kind {
	case "AWSManagedCluster":
		return true, "EKS"
	case "NutanixCluster":
		return true, "Nutanix"
	default:
		return false, ""
	}
}
