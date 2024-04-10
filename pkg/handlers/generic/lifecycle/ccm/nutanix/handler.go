// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	lifecycleutils "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/utils"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	defaultHelmReleaseName      = "nutanix-ccm"
	defaultHelmReleaseNamespace = "kube-system"

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
	clusterConfig *v1alpha1.ClusterConfigSpec,
) error {
	// No need to check for nil values in the struct, this function will only be called if CCM is not nil
	if clusterConfig.Addons.CCM.Credentials == nil {
		return ErrMissingCredentials
	}

	valuesTemplateConfigMap, err := lifecycleutils.RetrieveValuesTemplateConfigMap(
		ctx,
		p.client,
		p.config.defaultValuesTemplateConfigMapName,
		p.config.DefaultsNamespace(),
	)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve Nutanix CCM installation values template ConfigMap for cluster: %w",
			err,
		)
	}

	// It's possible to have the credentials Secret be created by the Helm chart.
	// However, that would leave the credentials visible in the HelmChartProxy.
	// Instead, we'll create the Secret on the remote cluster and reference it in the Helm values.
	if clusterConfig.Addons.CCM.Credentials != nil {
		key := ctrlclient.ObjectKey{
			Name:      defaultCredentialsSecretName,
			Namespace: defaultHelmReleaseNamespace,
		}
		err = lifecycleutils.CopySecretToRemoteCluster(
			ctx,
			p.client,
			clusterConfig.Addons.CCM.Credentials.Name,
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

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	helmChart, err := p.helmChartInfoGetter.For(ctx, log, config.NutanixCCM)
	if err != nil {
		return fmt.Errorf("failed to get values for nutanix-ccm-config %w", err)
	}

	values := valuesTemplateConfigMap.Data["values.yaml"]
	// The configMap will contain the Helm values, but templated with fields that need to be filled in.
	values, err = templateValues(clusterConfig, values)
	if err != nil {
		return fmt.Errorf("failed to template Helm values read from ConfigMap: %w", err)
	}

	hcp := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "nutanix-ccm-" + cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   helmChart.Repository,
			ChartName: helmChart.Name,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
			ReleaseNamespace: defaultHelmReleaseNamespace,
			ReleaseName:      defaultHelmReleaseName,
			Version:          helmChart.Version,
			ValuesTemplate:   values,
		},
	}

	if err = controllerutil.SetOwnerReference(cluster, hcp, p.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on nutanix-ccm installation HelmChartProxy: %w",
			err,
		)
	}

	if err = client.ServerSideApply(ctx, p.client, hcp); err != nil {
		return fmt.Errorf("failed to apply nutanix-ccm installation HelmChartProxy: %w", err)
	}

	return nil
}

func templateValues(clusterConfig *v1alpha1.ClusterConfigSpec, text string) (string, error) {
	helmValuesTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse Helm values template: %w", err)
	}

	type input struct {
		PrismCentralHost                  string
		PrismCentralPort                  int32
		PrismCentralInsecure              bool
		PrismCentralAdditionalTrustBundle *string
	}

	address, port, err := clusterConfig.Nutanix.PrismCentralEndpoint.ParseURL()
	if err != nil {
		return "", err
	}
	templateInput := input{
		PrismCentralHost:                  address,
		PrismCentralPort:                  port,
		PrismCentralInsecure:              clusterConfig.Nutanix.PrismCentralEndpoint.Insecure,
		PrismCentralAdditionalTrustBundle: clusterConfig.Nutanix.PrismCentralEndpoint.AdditionalTrustBundle,
	}

	var b bytes.Buffer
	err = helmValuesTemplate.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf("failed setting PrismCentral configuration in template: %w", err)
	}

	return b.String(), nil
}
