// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package distribution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	netutils "k8s.io/utils/net"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	DefaultHelmReleaseName      = "in-cluster"
	DefaultHelmReleaseNamespace = "registry-system"
)

type Config struct {
	*options.GlobalOptions

	defaultValuesTemplateConfigMapName string
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-registry-distribution-helm-values-template",
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
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Applying distribution registry installation")

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

	helmChartInfo, err := n.helmChartInfoGetter.For(ctx, log, config.RegistryDistribution)
	if err != nil {
		return fmt.Errorf("failed to get distribution registry helm chart: %w", err)
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
		return fmt.Errorf("failed to apply distribution registry addon: %w", err)
	}

	// FIXME: All of should be behind some API
	log.Info(
		fmt.Sprintf("Applying distribution registry loader objects to cluster %s",
			ctrlclient.ObjectKeyFromObject(cluster),
		),
	)
	configurationInput := &LoaderInput{
		Cluster: cluster,
	}
	cos, err := RegistryLoaderObjects(configurationInput)
	if err != nil {
		return fmt.Errorf("failed to generate distribution registry loader objects: %w", err)
	}
	err = applyObjs(ctx, remoteClient, cos)
	if err != nil {
		return fmt.Errorf("failed to apply distribution registry configuration: %w", err)
	}

	log.Info(
		fmt.Sprintf("Applying utility to seed image bundles for cluster %s",
			ctrlclient.ObjectKeyFromObject(cluster),
		),
	)

	// FIXME: CAREN is probably not the right place to do this
	cos, err = ManagementClusterObjects(configurationInput)
	if err != nil {
		return fmt.Errorf("failed to generate utility to seed image configuration: %w", err)
	}
	err = applyObjs(ctx, n.client, cos)
	if err != nil {
		return fmt.Errorf("failed to apply utility to seed image configuration: %w", err)
	}
	err = addClusterOwnerReferenceToObjs(ctx, n.client, cluster, cos)
	if err != nil {
		return fmt.Errorf("failed to add owner to seed image configuration: %w", err)
	}

	return nil
}

func templateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	valuesTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	serviceIP, err := getServiceIP(cluster.Spec.ClusterNetwork.Services.CIDRBlocks)
	if err != nil {
		return "", fmt.Errorf("error getting service IP for the mindthegap registry: %w", err)
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

func getServiceIP(serviceSubnetStrings []string) (string, error) {
	if len(serviceSubnetStrings) == 0 {
		serviceSubnetStrings = []string{v1alpha1.DefaultServicesSubnet}
	}

	serviceSubnets, err := netutils.ParseCIDRs(serviceSubnetStrings)
	if err != nil {
		return "", fmt.Errorf("unable to parse service Subnets: %w", err)
	}
	if len(serviceSubnets) == 0 {
		return "", errors.New("unexpected empty service Subnets")
	}

	// Selects the 20th IP in service subnet CIDR range as the Service IP
	serviceIP, err := netutils.GetIndexedIP(serviceSubnets[0], 20)
	if err != nil {
		return "", fmt.Errorf("unable to get internal Kubernetes Service IP from the given service Subnets")
	}

	return serviceIP.String(), nil
}

func applyObjs(ctx context.Context, cl ctrlclient.Client, objs []unstructured.Unstructured) error {
	for i := range objs {
		o := &objs[i]
		err := client.ServerSideApply(
			ctx,
			cl,
			o,
			&ctrlclient.PatchOptions{
				Raw: &metav1.PatchOptions{
					FieldValidation: metav1.FieldValidationStrict,
				},
			},
		)
		if err != nil {
			return fmt.Errorf(
				"failed to apply object %s %s: %w",
				o.GetKind(),
				ctrlclient.ObjectKeyFromObject(o),
				err,
			)
		}
	}

	return nil
}

func addClusterOwnerReferenceToObjs(
	ctx context.Context,
	cl ctrlclient.Client,
	cluster *clusterv1.Cluster,
	objs []unstructured.Unstructured,
) error {
	for i := range objs {
		o := &objs[i]
		err := handlersutils.EnsureClusterOwnerReferenceForObject(
			ctx,
			cl,
			corev1.TypedLocalObjectReference{
				APIGroup: ptr.To(o.GetAPIVersion()),
				Kind:     o.GetKind(),
				Name:     o.GetName(),
			},
			cluster,
		)
		if err != nil {
			return fmt.Errorf("failed to set owner reference for object %s %s: %w",
				o.GetKind(),
				ctrlclient.ObjectKeyFromObject(o),
				err,
			)
		}
	}

	return nil
}
