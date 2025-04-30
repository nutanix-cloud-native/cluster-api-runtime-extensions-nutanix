// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultHelmReleaseName      = "vsphere-cpi"
	defaultHelmReleaseNamespace = metav1.NamespaceSystem

	// This is the name of the Secret on the remote cluster that should match what is defined in Helm values.
	//nolint:gosec // Does not contain hard coded credentials.
	defaultCredentialsSecretName = "cloud-provider-vsphere-credentials"
)

var ErrMissingCredentials = errors.New(
	"name of the Secret containing vSphere credentials for CCM must be set",
)

type Config struct {
	*options.GlobalOptions

	defaultValuesTemplateConfigMapName string
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-vsphere-ccm-helm-values-template",
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

	helmChart, err := p.helmChartInfoGetter.For(ctx, log, config.VsphereCCM)
	if err != nil {
		return fmt.Errorf("failed to get values for vsphere-ccm-config: %w", err)
	}

	applier := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			p.config.defaultValuesTemplateConfigMapName,
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
		p.client,
		helmChart,
	)

	if err = applier.Apply(ctx, cluster, p.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply vsphere-ccm installation HelmChartProxy: %w", err)
	}

	return nil
}
