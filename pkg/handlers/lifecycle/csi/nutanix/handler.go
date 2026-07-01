// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	csiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/csi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	metroFailureDomainPrefix     = "NutanixMetro/"
	metroSiteFailureDomainPrefix = "NutanixMetroSite/"
)

const (
	defaultHelmReleaseName      = "nutanix-csi"
	defaultHelmReleaseNamespace = "ntnx-system"

	//nolint:gosec // Does not contain hard coded credentials.
	defaultCredentialsSecretName = "nutanix-csi-credentials"
)

var DefaultStorageClassParameters = map[string]string{
	"storageType":                                           "NutanixVolumes",
	"csi.storage.k8s.io/fstype":                             "xfs",
	"csi.storage.k8s.io/provisioner-secret-name":            defaultCredentialsSecretName,
	"csi.storage.k8s.io/provisioner-secret-namespace":       defaultHelmReleaseNamespace,
	"csi.storage.k8s.io/node-publish-secret-name":           defaultCredentialsSecretName,
	"csi.storage.k8s.io/node-publish-secret-namespace":      defaultHelmReleaseNamespace,
	"csi.storage.k8s.io/controller-expand-secret-name":      defaultCredentialsSecretName,
	"csi.storage.k8s.io/controller-expand-secret-namespace": defaultHelmReleaseNamespace,
}

type Config struct {
	*options.GlobalOptions

	helmAddonConfig *addons.HelmAddonConfig
}

func NewConfig(globalOptions *options.GlobalOptions) *Config {
	return &Config{
		GlobalOptions: globalOptions,
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-nutanix-csi-helm-values-template",
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
	}
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type NutanixCSI struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter
}

func New(
	c ctrlclient.Client,
	cfg *Config,
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
	var strategy addons.Applier
	switch provider.Strategy {
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := n.helmChartInfoGetter.For(ctx, log, config.NutanixStorageCSI)
		if err != nil {
			return fmt.Errorf(
				"failed to get configuration for Nutanix storage chart to create helm addon: %w",
				err,
			)
		}
		strategy = addons.NewHelmAddonApplier(
			n.config.helmAddonConfig,
			n.client,
			helmChart,
		).WithValueTemplater(templateValuesFunc(cluster))
	case "":
		return fmt.Errorf("strategy not provided for Nutanix CSI driver")
	default:
		return fmt.Errorf("strategy %s not implemented", provider.Strategy)
	}

	if provider.Credentials != nil {
		err := handlersutils.EnsureClusterOwnerReferenceForObject(
			ctx,
			n.client,
			corev1.TypedLocalObjectReference{
				Kind: "Secret",
				Name: provider.Credentials.SecretRef.Name,
			},
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
			Namespace: defaultHelmReleaseNamespace,
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

	// The Nutanix CSI driver requires hostPath volumes and other privileged
	// pod features that fail to admit under baseline or stricter Pod Security
	// Admission, so the workload-cluster namespace it runs in must be labelled
	// privileged.
	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		n.client,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return fmt.Errorf("error creating remote cluster client: %w", err)
	}
	if err := handlersutils.EnsureNamespaceWithMetadata(
		ctx,
		remoteClient,
		defaultHelmReleaseNamespace,
		handlersutils.PrivilegedPodSecurityEnforceLabels,
		nil,
	); err != nil {
		return fmt.Errorf(
			"failed to ensure %q namespace on the remote cluster: %w",
			defaultHelmReleaseNamespace,
			err,
		)
	}

	if err := strategy.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply nutanix CSI addon: %w", err)
	}

	err = csiutils.CreateStorageClassesOnRemote(
		ctx,
		n.client,
		provider.StorageClassConfigs,
		cluster,
		defaultStorage,
		v1alpha1.CSIProviderNutanix,
		v1alpha1.NutanixProvisioner,
		DefaultStorageClassParameters,
	)
	if err != nil {
		return fmt.Errorf("error creating StorageClasses for the Nutanix CSI driver: %w", err)
	}
	return nil
}

func templateValuesFunc(
	cluster *clusterv1.Cluster,
) func(*clusterv1.Cluster, string) (string, error) {
	return func(_ *clusterv1.Cluster, valuesTemplate string) (string, error) {
		helmValuesTemplate, err := template.New("").Parse(valuesTemplate)
		if err != nil {
			return "", fmt.Errorf("failed to parse Helm values template: %w", err)
		}

		type input struct {
			ApplyMpioConfigs bool
		}

		templateInput := input{
			ApplyMpioConfigs: isMetroCluster(cluster),
		}

		var b bytes.Buffer
		if err = helmValuesTemplate.Execute(&b, templateInput); err != nil {
			return "", fmt.Errorf("failed to template Nutanix CSI Helm values: %w", err)
		}

		return b.String(), nil
	}
}

// isMetroCluster returns true when the cluster uses metro-aware failure domains,
// i.e. any control-plane or worker failure domain references a NutanixMetro or
// NutanixMetroSite object (identified by the respective name prefix).
func isMetroCluster(cluster *clusterv1.Cluster) bool {
	if !cluster.Spec.Topology.IsDefined() {
		return false
	}

	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)
	clusterConfigVar, err := variables.Get[apivariables.ClusterConfigSpec](
		varMap,
		v1alpha1.ClusterConfigVariableName,
	)
	if err == nil &&
		clusterConfigVar.ControlPlane != nil &&
		clusterConfigVar.ControlPlane.Nutanix != nil {
		for _, fd := range clusterConfigVar.ControlPlane.Nutanix.FailureDomains {
			if strings.HasPrefix(fd, metroFailureDomainPrefix) ||
				strings.HasPrefix(fd, metroSiteFailureDomainPrefix) {
				return true
			}
		}
	}

	for i := range cluster.Spec.Topology.Workers.MachineDeployments {
		fd := cluster.Spec.Topology.Workers.MachineDeployments[i].FailureDomain
		if strings.HasPrefix(fd, metroFailureDomainPrefix) ||
			strings.HasPrefix(fd, metroSiteFailureDomainPrefix) {
			return true
		}
	}

	return false
}
