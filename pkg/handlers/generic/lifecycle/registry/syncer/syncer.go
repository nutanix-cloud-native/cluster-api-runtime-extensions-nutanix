// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package syncer

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	_ "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/feature"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/registry/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultHelmReleaseName = "registry-syncer"

	// defaultValuesTemplateConfigMapName is intentionally not exposed as a flag,
	// as this functionality is behind a feature flag.
	defaultValuesTemplateConfigMapName = "default-registry-syncer-helm-values-template"
)

type Config struct {
	*options.GlobalOptions
}

type RegistrySyncer struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter
}

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *RegistrySyncer {
	return &RegistrySyncer{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

// Apply applies the registry syncer on the target cluster.
func (n *RegistrySyncer) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Checking if registry syncer needs to be applied")
	managementCluster, err := capiutils.ManagementCluster(ctx, n.client)
	if err != nil {
		return fmt.Errorf("failed to get management cluster: %w", err)
	}
	shouldApply, err := shouldApplyRegistrySyncer(cluster, managementCluster)
	if err != nil {
		return fmt.Errorf("failed to check if registry syncer should be applied: %w", err)
	}
	if !shouldApply {
		log.Info("Skipping registry syncer as it is not required")
		return nil
	}

	log.Info("Applying registry syncer for cluster")
	helmChartInfo, err := n.helmChartInfoGetter.For(ctx, log, config.RegistrySyncer)
	if err != nil {
		return fmt.Errorf("failed to get registry syncer helm chart: %w", err)
	}

	addonApplier := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			defaultValuesTemplateConfigMapName,
			cluster.Namespace,
			defaultHelmReleaseName,
		),
		n.client,
		helmChartInfo,
	).
		// Always deploy this on the management cluster and sync to the workload cluster.
		WithTargetCluster(managementCluster).
		WithValueTemplater(templateValues).
		WithHelmReleaseName(addonResourceNameForCluster(cluster))

	if err := addonApplier.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply registry syncer addon: %w", err)
	}

	return nil
}

// Cleanup cleans up the HCP on the management cluster for the cluster.
// The syncer is applied against the management cluster can be in a different namespace from cluster.
// Since cross-namespace owner references are not allowed, we need to delete the HelmChartProxy directly.
func (n *RegistrySyncer) Cleanup(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Checking if registry syncer needs to be cleaned up")
	managementCluster, err := capiutils.ManagementCluster(ctx, n.client)
	if err != nil {
		return fmt.Errorf("failed to get management cluster: %w", err)
	}
	shouldApply, err := shouldApplyRegistrySyncer(cluster, managementCluster)
	if err != nil {
		return fmt.Errorf("failed to check if registry syncer should be cleaned up: %w", err)
	}
	if !shouldApply {
		log.Info("Skipping registry syncer cleanup as it is not required")
		return nil
	}

	log.Info("Cleaning up registry syncer for cluster")
	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addonResourceNameForCluster(cluster),
			Namespace: managementCluster.Namespace,
		},
	}

	err = ctrlclient.IgnoreNotFound(n.client.Delete(ctx, hcp))
	if err != nil {
		return fmt.Errorf(
			"failed to delete regystry syncer installation HelmChartProxy: %w",
			err,
		)
	}

	return nil
}

// shouldApply returns false if:
// - the management cluster is nil, ie. this cluster will become the management cluster
// - the management cluster name and namespace matches the workload cluster
// - the feature gate for synchronizing workload cluster registry is not enabled
// - the cluster has the SkipSynchronizingWorkloadClusterRegistry annotation
// - the registry addon is not enabled in the cluster
// - the registry addon is not enabled in the management cluster
// Otherwise, it returns true.
func shouldApplyRegistrySyncer(cluster, managementCluster *clusterv1.Cluster) (bool, error) {
	if managementCluster == nil {
		return false, nil
	}

	if managementCluster.Name == cluster.Name &&
		managementCluster.Namespace == cluster.Namespace {
		return false, nil
	}

	if !feature.Gates.Enabled(feature.SynchronizeWorkloadClusterRegistry) {
		return false, nil
	}

	if hasRegistrySyncerSkipAnnotation(cluster) {
		return false, nil
	}

	clusterAddon, err := variables.RegistryAddon(cluster)
	if err != nil {
		return false, fmt.Errorf("failed to check if registry addon is enabled in the cluster: %w", err)
	}
	if clusterAddon == nil {
		return false, nil
	}

	managementClusterAddon, err := variables.RegistryAddon(managementCluster)
	if err != nil {
		return false, fmt.Errorf("failed to check if registry addon is enabled in management cluster: %w", err)
	}
	if managementClusterAddon == nil {
		return false, nil
	}

	return true, nil
}

func hasRegistrySyncerSkipAnnotation(cluster *clusterv1.Cluster) bool {
	if cluster.Annotations == nil {
		return false
	}
	_, ok := cluster.Annotations[carenv1.SkipSynchronizingWorkloadClusterRegistry]
	return ok
}

func templateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	valuesTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	registryMetadata, err := utils.GetRegistryMetadata(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to get registry metadata: %w", err)
	}

	type input struct {
		CusterName string

		KubernetesVersion string

		SourceRegistryAddress string

		DestinationRegistryAnyPodName               string
		DestinationRegistryHeadlessServiceNamespace string
		DestinationRegistryHeadlessServiceName      string
		DestinationRegistryHeadlessServicePort      int32

		RegistryCASecretName string
	}

	templateInput := input{
		CusterName:        cluster.Name,
		KubernetesVersion: cluster.Spec.Topology.Version,
		// FIXME: This assumes that the source and destination registry names are the same.
		// This is true now with a single registry addon provider, but may not be true in the future.
		SourceRegistryAddress:                       registryMetadata.AddressFromClusterNetwork,
		DestinationRegistryAnyPodName:               registryMetadata.AnyPodName,
		DestinationRegistryHeadlessServiceNamespace: registryMetadata.Namespace,
		DestinationRegistryHeadlessServiceName:      registryMetadata.HeadlessServiceName,
		DestinationRegistryHeadlessServicePort:      registryMetadata.HeadlessServicePort,
		RegistryCASecretName:                        handlersutils.SecretNameForRegistryAddonCA(cluster),
	}

	var b bytes.Buffer
	err = valuesTemplate.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf(
			"failed template values: %w",
			err,
		)
	}

	return b.String(), nil
}

func addonResourceNameForCluster(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-%s", defaultHelmReleaseName, cluster.Annotations[carenv1.ClusterUUIDAnnotationKey])
}
