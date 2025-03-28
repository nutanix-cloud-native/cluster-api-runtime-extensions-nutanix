// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultHelmReleaseName      = "nutanix-ccm"
	defaultHelmReleaseNamespace = metav1.NamespaceSystem

	// This is the name of the Secret on the remote cluster that should match what is defined in Helm values.
	//nolint:gosec // Does not contain hard coded credentials.
	defaultCredentialsSecretName = "nutanix-ccm-credentials"
)

var ErrMissingCredentials = errors.New("name of the Secret containing PC credentials must be set")

type Config struct {
	*options.GlobalOptions

	defaultValuesTemplateConfigMapName string
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-nutanix-ccm-helm-values-template",
		"default values ConfigMap name",
	)
}

type provider struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter
}

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *provider {
	return &provider{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (p *provider) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	clusterConfig *apivariables.ClusterConfigSpec,
	log logr.Logger,
) error {
	// No need to check for nil values in the struct, this function will only be called if CCM is not nil
	if clusterConfig.Addons.CCM.Credentials == nil {
		ccmCredentialsRequest := NutanixCCMCreentialsRequest(
			cluster.Name,
			cluster.Namespace,
			defaultCredentialsSecretName,
			defaultHelmReleaseNamespace)
		if err := handlersutils.CreateNutanixCredentialsRequest(ctx, p.client, ccmCredentialsRequest); err != nil {
			return fmt.Errorf("error creating Nutanix CCM Credentials Request: %w", err)
		}
	}

	// It's possible to have the credentials Secret be created by the Helm chart.
	// However, that would leave the credentials visible in the HelmChartProxy.
	// Instead, we'll create the Secret on the remote cluster and reference it in the Helm values.
	if clusterConfig.Addons.CCM.Credentials != nil {
		err := handlersutils.EnsureClusterOwnerReferenceForObject(
			ctx,
			p.client,
			corev1.TypedLocalObjectReference{
				Kind: "Secret",
				Name: clusterConfig.Addons.CCM.Credentials.SecretRef.Name,
			},
			cluster,
		)
		if err != nil {
			return fmt.Errorf(
				"error updating owner references on Nutanix CCM source Secret: %w",
				err,
			)
		}
		key := ctrlclient.ObjectKey{
			Name:      defaultCredentialsSecretName,
			Namespace: defaultHelmReleaseNamespace,
		}
		err = handlersutils.CopySecretToRemoteCluster(
			ctx,
			p.client,
			clusterConfig.Addons.CCM.Credentials.SecretRef.Name,
			key,
			cluster,
		)
		if err != nil {
			return fmt.Errorf(
				"error creating Nutanix CCM Credentials Secret on the remote cluster: %w",
				err,
			)
		}
	}

	helmChart, err := p.helmChartInfoGetter.For(ctx, log, config.NutanixCCM)
	if err != nil {
		return fmt.Errorf("failed to get values for nutanix-ccm-config: %w", err)
	}

	applier := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			p.config.defaultValuesTemplateConfigMapName,
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
		p.client,
		helmChart,
	).WithValueTemplater(templateValuesFunc(clusterConfig.Nutanix))

	if err = applier.Apply(ctx, cluster, p.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply nutanix-ccm installation HelmChartProxy: %w", err)
	}

	return nil
}

func templateValuesFunc(
	nutanixConfig *v1alpha1.NutanixSpec,
) func(*clusterv1.Cluster, string) (string, error) {
	return func(_ *clusterv1.Cluster, valuesTemplate string) (string, error) {
		joinQuoted := template.FuncMap{
			"joinQuoted": func(items []string) string {
				for i, item := range items {
					items[i] = fmt.Sprintf("%q", item)
				}
				return strings.Join(items, ", ")
			},
		}
		helmValuesTemplate, err := template.New("").Funcs(joinQuoted).Parse(valuesTemplate)
		if err != nil {
			return "", fmt.Errorf("failed to parse Helm values template: %w", err)
		}

		type input struct {
			PrismCentralHost                  string
			PrismCentralPort                  uint16
			PrismCentralInsecure              bool
			PrismCentralAdditionalTrustBundle string
			IPsToIgnore                       []string
		}

		address, port, err := nutanixConfig.PrismCentralEndpoint.ParseURL()
		if err != nil {
			return "", err
		}
		templateInput := input{
			PrismCentralHost:                  address,
			PrismCentralPort:                  port,
			PrismCentralInsecure:              nutanixConfig.PrismCentralEndpoint.Insecure,
			PrismCentralAdditionalTrustBundle: nutanixConfig.PrismCentralEndpoint.AdditionalTrustBundle,
			IPsToIgnore:                       ipsToIgnore(nutanixConfig),
		}

		var b bytes.Buffer
		err = helmValuesTemplate.Execute(&b, templateInput)
		if err != nil {
			return "", fmt.Errorf("failed setting PrismCentral configuration in template: %w", err)
		}

		return b.String(), nil
	}
}

func ipsToIgnore(nutanixConfig *v1alpha1.NutanixSpec) []string {
	toIgnore := []string{nutanixConfig.ControlPlaneEndpoint.Host}
	// Also ignore the virtual IP if it is set.
	if nutanixConfig.ControlPlaneEndpoint.VirtualIPSpec != nil &&
		nutanixConfig.ControlPlaneEndpoint.VirtualIPSpec.Configuration != nil &&
		nutanixConfig.ControlPlaneEndpoint.VirtualIPSpec.Configuration.Address != "" {
		toIgnore = append(
			toIgnore,
			nutanixConfig.ControlPlaneEndpoint.VirtualIPSpec.Configuration.Address,
		)
	}
	return toIgnore
}
