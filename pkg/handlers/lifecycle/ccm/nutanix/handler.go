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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/serviceloadbalancer/metallb"
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
		return ErrMissingCredentials
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
	).WithValueTemplater(
		templateValuesFunc(clusterConfig.Nutanix, clusterConfig.Addons.ServiceLoadBalancer),
	)

	if err = applier.Apply(ctx, cluster, p.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply nutanix-ccm installation HelmChartProxy: %w", err)
	}

	return nil
}

// autoIPAMConfig returns the automatic Service address configuration when the
// cluster uses it. MetalLB is the only provider that advertises the pools the
// CCM creates, so any other provider leaves the Service controller off.
func autoIPAMConfig(
	slb *v1alpha1.ServiceLoadBalancer,
) *v1alpha1.ServiceLoadBalancerAutoIPAM {
	if slb == nil ||
		slb.Provider != v1alpha1.ServiceLoadBalancerProviderMetalLB ||
		slb.Configuration == nil {
		return nil
	}
	return slb.Configuration.AutoIPAM
}

// clusterUID is the cluster half of the reservation context the CCM stores on
// Prism Central. An empty value renders empty and the CCM refuses to reserve,
// rather than reserving an address no cluster teardown could release.
func clusterUID(cluster *clusterv1.Cluster) string {
	if cluster == nil {
		return ""
	}
	return string(cluster.GetUID())
}

// controlPlaneEndpoint is the endpoint the CCM is configured with. The cluster
// config carries no host while the address is reserved from an IPAM pool - the
// reserved address is resolved into the generated templates, never persisted -
// so it is taken from the Cluster, which CAPI has populated by the time any
// addon is installed.
//
// An empty host must not reach the values template: it renders
// KUBERNETES_SERVICE_HOST="", which defeats the in-cluster config the CCM would
// otherwise fall back to, and the CCM then crash-loops with the nodes still
// carrying the uninitialized taint.
func controlPlaneEndpoint(
	cluster *clusterv1.Cluster,
	spec v1alpha1.ControlPlaneEndpointSpec,
) (v1alpha1.ControlPlaneEndpointSpec, error) {
	if spec.Host == "" && cluster != nil {
		spec.Host = cluster.Spec.ControlPlaneEndpoint.Host
	}
	if spec.Port == 0 && cluster != nil {
		spec.Port = cluster.Spec.ControlPlaneEndpoint.Port
	}
	if spec.Host == "" {
		return spec, fmt.Errorf(
			"no control plane endpoint host for cluster %s; "+
				"if the address is reserved from a subnet, the reservation has not completed yet",
			ctrlclient.ObjectKeyFromObject(cluster),
		)
	}
	return spec, nil
}

func templateValuesFunc(
	nutanixConfig *v1alpha1.NutanixSpec,
	serviceLoadBalancer *v1alpha1.ServiceLoadBalancer,
) func(*clusterv1.Cluster, string) (string, error) {
	return func(cluster *clusterv1.Cluster, valuesTemplate string) (string, error) {
		joinQuoted := template.FuncMap{
			"joinQuoted": func(items []string) string {
				for i, item := range items {
					items[i] = fmt.Sprintf("%q", item)
				}
				return strings.Join(items, ", ")
			},
			"trimPrefix": strings.TrimPrefix,
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
			ControlPlaneEndpoint              v1alpha1.ControlPlaneEndpointSpec

			// ServiceLoadBalancerAutoIPAM is nil unless the cluster reserves
			// Service addresses automatically. It is the only switch for the CCM
			// LoadBalancer strategy, so the two cannot disagree.
			ServiceLoadBalancerAutoIPAM *v1alpha1.ServiceLoadBalancerAutoIPAM
			// MetalLBNamespace is where the CCM creates its per-Service pools. It
			// must be the namespace CAREN installs MetalLB into.
			MetalLBNamespace string
			// ClusterUID identifies this cluster in the reservation context the
			// CCM stores on Prism Central. It is what lets the management cluster
			// release this cluster's addresses once the cluster is gone, so it
			// must be the Cluster UID and not a name, which is reusable.
			ClusterUID string
		}

		address, port, err := nutanixConfig.PrismCentralEndpoint.ParseURL()
		if err != nil {
			return "", err
		}

		endpoint, err := controlPlaneEndpoint(cluster, nutanixConfig.ControlPlaneEndpoint)
		if err != nil {
			return "", err
		}

		templateInput := input{
			PrismCentralHost:                  address,
			PrismCentralPort:                  port,
			PrismCentralInsecure:              nutanixConfig.PrismCentralEndpoint.Insecure,
			PrismCentralAdditionalTrustBundle: nutanixConfig.PrismCentralEndpoint.AdditionalTrustBundle,
			IPsToIgnore:                       ipsToIgnore(endpoint),
			ControlPlaneEndpoint:              endpoint,
			ServiceLoadBalancerAutoIPAM:       autoIPAMConfig(serviceLoadBalancer),
			MetalLBNamespace:                  metallb.DefaultHelmReleaseNamespace,
			ClusterUID:                        clusterUID(cluster),
		}

		var b bytes.Buffer
		err = helmValuesTemplate.Execute(&b, templateInput)
		if err != nil {
			return "", fmt.Errorf("failed setting PrismCentral configuration in template: %w", err)
		}

		return b.String(), nil
	}
}

func ipsToIgnore(endpoint v1alpha1.ControlPlaneEndpointSpec) []string {
	// An empty string in this list would be rendered into the CCM's
	// ignoredNodeIPs and compared against every node address.
	toIgnore := []string{}
	if host := endpoint.Host; host != "" {
		toIgnore = append(toIgnore, host)
	}
	// Also ignore the virtual IP if it is set and differs from the host, which
	// it does not when both come from the same reservation.
	if endpoint.VirtualIPSpec != nil &&
		endpoint.VirtualIPSpec.Configuration != nil &&
		endpoint.VirtualIPSpec.Configuration.Address != "" &&
		endpoint.VirtualIPSpec.Configuration.Address != endpoint.Host {
		toIgnore = append(toIgnore, endpoint.VirtualIPSpec.Configuration.Address)
	}
	return toIgnore
}
