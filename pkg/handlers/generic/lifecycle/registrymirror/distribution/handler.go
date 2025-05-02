// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package distribution

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/registrymirror/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	DefaultHelmReleaseName      = "registry-mirror"
	DefaultHelmReleaseNamespace = "registry-mirror-system"
)

type Config struct {
	*options.GlobalOptions

	defaultValuesTemplateConfigMapName string
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-distribution-registry-mirror-helm-values-template",
		"default values ConfigMap name",
	)
}

type Distribution struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter
}

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *Distribution {
	return &Distribution{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (n *Distribution) Apply(
	ctx context.Context,
	_ v1alpha1.RegistryMirror,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Applying Distribution registry mirror installation")

	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		n.client,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return fmt.Errorf("error creating remote cluster client: %w", err)
	}

	err = handlersutils.EnsureNamespaceWithMetadata(
		ctx,
		remoteClient,
		DefaultHelmReleaseNamespace,
		nil,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to ensure release namespace %q exists: %w",
			DefaultHelmReleaseName,
			err,
		)
	}

	helmChartInfo, err := n.helmChartInfoGetter.For(ctx, log, config.DistributionRegistryMirror)
	if err != nil {
		return fmt.Errorf("failed to get Distribution registry mirror helm chart: %w", err)
	}

	addonApplier := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			n.config.defaultValuesTemplateConfigMapName,
			DefaultHelmReleaseNamespace,
			DefaultHelmReleaseName,
		),
		n.client,
		helmChartInfo,
	).WithDefaultWaiter().WithValueTemplater(templateValues)

	if err := addonApplier.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply Distribution registry mirror addon: %w", err)
	}

	return nil
}

func templateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	valuesTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	serviceIP, err := utils.ServiceIPForCluster(cluster)
	if err != nil {
		return "", fmt.Errorf("error getting service IP for the distribution registry: %w", err)
	}

	type input struct {
		ServiceIP string
	}

	templateInput := input{
		ServiceIP: serviceIP,
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
