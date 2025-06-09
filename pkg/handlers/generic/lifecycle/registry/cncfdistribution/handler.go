// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cncfdistribution

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/registry/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	DefaultHelmReleaseName      = "cncf-distribution-registry"
	DefaultHelmReleaseNamespace = "registry-system"

	stsName                = "cncf-distribution-registry-docker-registry"
	stsHeadlessServiceName = "cncf-distribution-registry-docker-registry-headless"
	stsReplicas            = 2
	tlsSecretName          = "registry-tls"
)

type Config struct {
	*options.GlobalOptions

	defaultValuesTemplateConfigMapName string
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-cncf-distribution-registry-helm-values-template",
		"default values ConfigMap name",
	)
}

type CNCFDistribution struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter
}

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *CNCFDistribution {
	return &CNCFDistribution{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

// Setup ensures any pre-requisites for the CNCF Distribution registry addon are met.
// It is expected to be called before the cluster is created.
// Specifically, it ensures that the CA secret for the registry is created in the cluster's namespace.
func (n *CNCFDistribution) Setup(
	ctx context.Context,
	_ v1alpha1.RegistryAddon,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Setting up root CA for CNCF Distribution registry if not already present")
	err := utils.EnsureRegistryAddonRootCASecret(ctx, n.client, cluster)
	if err != nil {
		return fmt.Errorf("failed to ensure root CA secret for CNCF Distribution registry addon: %w", err)
	}

	log.Info("Setting up CA for CNCF Distribution registry")
	err = utils.EnsureCASecretForCluster(
		ctx,
		n.client,
		cluster,
	)
	if err != nil {
		return fmt.Errorf("failed to ensure CA secret for CNCF Distribution registry addon: %w", err)
	}
	return nil
}

// Apply applies the CNCF Distribution registry addon to the cluster.
func (n *CNCFDistribution) Apply(
	ctx context.Context,
	_ v1alpha1.RegistryAddon,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	// Copy the TLS secret to the remote cluster.
	serviceIP, err := utils.ServiceIPForCluster(cluster)
	if err != nil {
		return fmt.Errorf("error getting service IP for the CNCF distribution registry: %w", err)
	}
	opts := &utils.EnsureCertificateOpts{
		RemoteSecretKey: ctrlclient.ObjectKey{
			Name:      tlsSecretName,
			Namespace: DefaultHelmReleaseNamespace,
		},
		Spec: utils.CertificateSpec{
			CommonName:  stsName,
			DNSNames:    certificateDNSNames(),
			IPAddresses: certificateIPAddresses(serviceIP),
		},
	}
	err = utils.EnsureRegistryServerCertificateSecretOnRemoteCluster(
		ctx,
		n.client,
		cluster,
		opts,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to copy certificate secret for CNCF Distribution registry addon to remote cluster: %w",
			err,
		)
	}

	log.Info("Applying CNCF Distribution registry installation")
	helmChartInfo, err := n.helmChartInfoGetter.For(ctx, log, config.CNCFDistributionRegistry)
	if err != nil {
		return fmt.Errorf("failed to get CNCF Distribution registry helm chart: %w", err)
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
		return fmt.Errorf("failed to apply CNCF Distribution registry addon: %w", err)
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
		return "", fmt.Errorf("error getting service IP for the CNCF distribution registry: %w", err)
	}

	type input struct {
		ServiceIP     string
		Replicas      int32
		TLSSecretName string
	}

	templateInput := input{
		Replicas:      stsReplicas,
		ServiceIP:     serviceIP,
		TLSSecretName: tlsSecretName,
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

func certificateDNSNames() []string {
	names := []string{
		stsName,
		fmt.Sprintf("%s.%s", stsName, DefaultHelmReleaseNamespace),
		fmt.Sprintf("%s.%s.svc", stsName, DefaultHelmReleaseNamespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", stsName, DefaultHelmReleaseNamespace),
	}
	for i := 0; i < stsReplicas; i++ {
		names = append(names,
			[]string{
				fmt.Sprintf("%s-%d", stsName, i),
				fmt.Sprintf("%s-%d.%s.%s", stsName, i, stsHeadlessServiceName, DefaultHelmReleaseNamespace),
				fmt.Sprintf("%s-%d.%s.%s.svc", stsName, i, stsHeadlessServiceName, DefaultHelmReleaseNamespace),
				fmt.Sprintf(
					"%s-%d.%s.%s.svc.cluster.local",
					stsName, i, stsHeadlessServiceName, DefaultHelmReleaseNamespace,
				),
			}...,
		)
	}

	return names
}

func certificateIPAddresses(serviceIP string) []string {
	return []string{
		serviceIP,
		"127.0.0.1",
	}
}
