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

func (n *CNCFDistribution) Setup(
	ctx context.Context,
	_ v1alpha1.RegistryAddon,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	serviceIP, err := utils.ServiceIPForCluster(cluster)
	if err != nil {
		return fmt.Errorf("error getting service IP for the CNCF distribution registry: %w", err)
	}

	log.Info("Setting up TLS for CNCF Distribution registry")
	opts := &utils.EnsureCertificateOpts{
		RemoteSecretName:      tlsSecretName,
		RemoteSecretNamespace: DefaultHelmReleaseNamespace,
		Spec: utils.CertificateSpec{
			CommonName:  stsName,
			DNSNames:    dnsNames(),
			IPAddresses: ipAddresses(serviceIP),
		},
	}
	err = utils.EnsureCertificate(
		ctx,
		n.client,
		cluster,
		opts,
	)
	if err != nil {
		return fmt.Errorf("failed to ensure certificate secret for CNCF Distribution registry addon: %w", err)
	}
	return nil
}

func (n *CNCFDistribution) Apply(
	ctx context.Context,
	_ v1alpha1.RegistryAddon,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Copying TLS Certificate for CNCF Distribution registry to remote cluster")
	remoteCertificateSecretKey := ctrlclient.ObjectKey{
		Name:      tlsSecretName,
		Namespace: DefaultHelmReleaseNamespace,
	}
	err := utils.EnsureCertificateOnRemoteCluster(
		ctx,
		n.client,
		cluster,
		remoteCertificateSecretKey,
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
		ServiceIP string
		Replicas  int32
	}

	templateInput := input{
		Replicas:  stsReplicas,
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

func dnsNames() []string {
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

func ipAddresses(serviceIP string) []string {
	return []string{
		serviceIP,
		"127.0.0.1",
	}
}
