// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallb

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/wait"
)

const (
	DefaultHelmReleaseName      = "metallb"
	DefaultHelmReleaseNamespace = "metallb-system"
)

// These labels allow the MetalLB speaker pod to obtain elevated permissions,
// which it requires in order to perform its network functionalities.
var podSecurityReleaseNamespaceLabels = map[string]string{
	"pod-security.kubernetes.io/enforce":         "privileged",
	"pod-security.kubernetes.io/enforce-version": "latest",
}

type Config struct {
	*options.GlobalOptions

	defaultValuesTemplateConfigMapName string
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-metallb-helm-values-template",
		"default values ConfigMap name",
	)
}

type MetalLB struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter
}

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *MetalLB {
	return &MetalLB{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (n *MetalLB) Apply(
	ctx context.Context,
	slb v1alpha1.ServiceLoadBalancer,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Applying MetalLB installation")

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
		podSecurityReleaseNamespaceLabels,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to ensure release namespace %q exists: %w",
			DefaultHelmReleaseName,
			err,
		)
	}

	helmChartInfo, err := n.helmChartInfoGetter.For(ctx, log, config.MetalLB)
	if err != nil {
		return fmt.Errorf("failed to get MetalLB helm chart: %w", err)
	}

	addonApplier := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			n.config.defaultValuesTemplateConfigMapName,
			DefaultHelmReleaseNamespace,
			DefaultHelmReleaseName,
		),
		n.client,
		helmChartInfo,
	)

	if err := addonApplier.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply MetalLB addon: %w", err)
	}

	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name: fmt.Sprintf(
				"%s-%s",
				DefaultHelmReleaseName,
				cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey],
			),
		},
	}

	if err := wait.ForObject(
		ctx,
		wait.ForObjectInput[*caaphv1.HelmChartProxy]{
			Reader: n.client,
			Target: hcp.DeepCopy(),
			Check: func(_ context.Context, obj *caaphv1.HelmChartProxy) (bool, error) {
				return conditions.IsTrue(obj, caaphv1.HelmReleaseProxiesReadyCondition), nil
			},
			Interval: 5 * time.Second,
			Timeout:  30 * time.Second,
		},
	); err != nil {
		return fmt.Errorf("failed to wait for MetalLB to deploy: %w", err)
	}

	if slb.Configuration == nil {
		// Nothing more to do.
		return nil
	}

	log.Info(
		fmt.Sprintf("Applying MetalLB configuration to cluster %s",
			ctrlclient.ObjectKeyFromObject(cluster),
		),
	)

	cos, err := ConfigurationObjects(&ConfigurationInput{
		Name:          DefaultHelmReleaseName,
		Namespace:     DefaultHelmReleaseNamespace,
		AddressRanges: slb.Configuration.AddressRanges,
	})
	if err != nil {
		return fmt.Errorf("failed to generate MetalLB configuration: %w", err)
	}

	var applyErr error
	if waitErr := kwait.PollUntilContextTimeout(
		ctx,
		2*time.Second,
		10*time.Second,
		true,
		func(ctx context.Context) (done bool, err error) {
			for i := range cos {
				o := cos[i]
				if err = client.ServerSideApply(
					ctx,
					remoteClient,
					o,
					&ctrlclient.PatchOptions{
						Raw: &metav1.PatchOptions{
							FieldValidation: metav1.FieldValidationStrict,
						},
					},
				); err != nil {
					applyErr = fmt.Errorf(
						"failed to apply MetalLB configuration %s %s: %w",
						o.GetKind(),
						ctrlclient.ObjectKeyFromObject(o),
						err,
					)
					return false, nil
				}
			}
			return true, nil
		},
	); waitErr != nil {
		if applyErr != nil {
			return fmt.Errorf("%w: last apply error: %w", waitErr, applyErr)
		}
		return fmt.Errorf("%w: failed to apply MetalLB configuration", waitErr)
	}

	return nil
}
