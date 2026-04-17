// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
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
	_ context.Context,
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
