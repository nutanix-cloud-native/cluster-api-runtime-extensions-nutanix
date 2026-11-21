// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	ciliumv2 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumv2alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	k8sclient "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	configurationName      = "caren"
	configurationNamespace = "kube-system"
)

type Config struct {
	*options.GlobalOptions
}

func (c *Config) AddFlags(_ string, _ *pflag.FlagSet) {}

type Cilium struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter
}

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *Cilium {
	return &Cilium{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (c *Cilium) Apply(
	ctx context.Context,
	slb v1alpha1.ServiceLoadBalancer,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Applying Cilium ServiceLoadBalancer configuration")

	if err := c.validatePrerequisites(cluster); err != nil {
		return err
	}

	if slb.Configuration == nil {
		return nil
	}

	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		c.client,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return fmt.Errorf("error creating remote cluster client: %w", err)
	}

	configInput := &ConfigurationInput{
		Name:          configurationName,
		Namespace:     configurationNamespace,
		AddressRanges: slb.Configuration.AddressRanges,
	}
	objs, err := ConfigurationObjects(configInput)
	if err != nil {
		return fmt.Errorf("failed to generate Cilium LB configuration: %w", err)
	}

	var applyErr error
	if waitErr := kwait.PollUntilContextTimeout(
		ctx,
		2*time.Second,
		10*time.Second,
		true,
		func(ctx context.Context) (bool, error) {
			for _, o := range objs {
				err := k8sclient.ServerSideApply(
					ctx,
					remoteClient,
					o,
					&ctrlclient.PatchOptions{
						Raw: &metav1.PatchOptions{
							FieldValidation: metav1.FieldValidationStrict,
						},
					},
				)

				switch {
				case err == nil:
					continue
				case apierrors.IsInternalError(err):
					// CRD webhooks may not yet be registered; retry.
					return false, nil
				case apierrors.IsConflict(err):
					switch o.(type) {
					case *ciliumv2.CiliumLoadBalancerIPPool:
						err = fmt.Errorf(
							"%w. This resource has been modified in the workload cluster: it must contain exactly the blocks listed in the Cluster configuration", //nolint:lll // Long error message
							err,
						)
					case *ciliumv2alpha1.CiliumL2AnnouncementPolicy:
						err = fmt.Errorf(
							"%w. This resource has been modified in the workload cluster: it must only announce LoadBalancer IPs", //nolint:lll // Long error message
							err,
						)
					}

					applyErr = fmt.Errorf(
						"failed to apply Cilium LB configuration %s %s: %w",
						o.GetObjectKind().GroupVersionKind().Kind,
						ctrlclient.ObjectKeyFromObject(o),
						err,
					)

					return false, nil
				default:
					return false, err
				}
			}

			return true, nil
		},
	); waitErr != nil {
		if applyErr != nil {
			return fmt.Errorf("%w: last apply error: %w", waitErr, applyErr)
		}
		return fmt.Errorf("failed to apply Cilium LB configuration: %w", waitErr)
	}

	return nil
}

// validatePrerequisites re-checks, at apply time, that the cluster is still
// configured with the Cilium CNI and kube-proxy disabled. The webhook enforces
// these at admission, but this is a defence-in-depth check so a misconfigured
// cluster surfaces a clear error instead of silently creating pool objects
// against a cluster that cannot announce them.
func (c *Cilium) validatePrerequisites(cluster *clusterv1.Cluster) error {
	spec, err := apivariables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cluster config: %w", err)
	}

	if spec == nil || spec.Addons == nil || spec.Addons.CNI == nil ||
		spec.Addons.CNI.Provider != v1alpha1.CNIProviderCilium {
		return fmt.Errorf(
			"ServiceLoadBalancer provider %q requires Cilium CNI (addons.cni.provider=Cilium)",
			v1alpha1.ServiceLoadBalancerProviderCilium,
		)
	}

	if spec.KubeProxy == nil || spec.KubeProxy.Mode != v1alpha1.KubeProxyModeDisabled {
		return fmt.Errorf(
			"ServiceLoadBalancer provider %q requires kube-proxy to be disabled (kubeProxy.mode=disabled)",
			v1alpha1.ServiceLoadBalancerProviderCilium,
		)
	}

	return nil
}
