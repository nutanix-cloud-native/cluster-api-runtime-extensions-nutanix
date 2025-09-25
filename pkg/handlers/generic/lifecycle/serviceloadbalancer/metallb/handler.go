// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallb

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	metallbv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/go.universe.tf/metallb/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
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
	).WithDefaultWaiter()

	if err := addonApplier.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		return fmt.Errorf("failed to apply MetalLB addon: %w", err)
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

	configInput := &ConfigurationInput{
		Name:          DefaultHelmReleaseName,
		Namespace:     DefaultHelmReleaseNamespace,
		AddressRanges: slb.Configuration.AddressRanges,
	}
	cos, err := ConfigurationObjects(configInput)
	if err != nil {
		return fmt.Errorf("failed to generate MetalLB configuration: %w", err)
	}

	var applyErr error
	if waitErr := kwait.PollUntilContextTimeout(
		ctx,
		2*time.Second,
		10*time.Second,
		true,
		func(ctx context.Context) (bool, error) {
			for _, o := range cos {
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
					if apierrors.IsInternalError(err) {
						// Retry on internal errors as these are generally seen when the necessary
						// CRD webhooks are not yet registered.
						return false, nil
					}

					// Return early if the error is not a conflict.
					if !apierrors.IsConflict(err) {
						return false, err
					}

					// At this point, we have handled both internal and non-conflict errors,
					// so we must be dealing with a conflict.

					// Set the error message based on the type of the object.
					switch o.(type) {
					case *metallbv1.IPAddressPool:
						err = fmt.Errorf(
							"%w. This resource has been modified in the workload cluster: it must contain exactly the addresses listed in the Cluster configuration", //nolint:lll // Long error message,
							err,
						)
					case *metallbv1.L2Advertisement:
						err = fmt.Errorf(
							"%w. This resource has been modified in the workload cluster, it must only contain the %q IP Address Pool", //nolint:lll // Long error message,
							err,
							configInput.Name,
						)
					}

					applyErr = fmt.Errorf(
						"failed to apply MetalLB configuration %s %s: %w",
						o.GetObjectKind().GroupVersionKind().Kind,
						ctrlclient.ObjectKeyFromObject(o),
						err,
					)

					// Return false with no error to retry the apply.
					return false, nil
				}
			}

			return true, nil
		},
	); waitErr != nil {
		if applyErr != nil {
			return fmt.Errorf("%w: last apply error: %w", waitErr, applyErr)
		}

		return fmt.Errorf("failed to apply MetalLB configuration: %w", waitErr)
	}

	return nil
}
