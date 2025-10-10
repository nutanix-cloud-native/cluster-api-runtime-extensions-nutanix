// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	defaultMultusReleaseName = "multus"
	defaultMultusNamespace   = metav1.NamespaceSystem

	// Wait parameters
	waitInterval = 5 * time.Second
	maxWaitTime  = 10 * time.Minute
)

type CNIConfig struct {
	*options.GlobalOptions

	helmAddonConfig helmAddonConfig
}

type helmAddonConfig struct {
	defaultValuesTemplateConfigMapName string
}

func (c *helmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-multus-cni-helm-values-template",
		"default values ConfigMap name",
	)
}

func (c *CNIConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type MultusCNI struct {
	client              ctrlclient.Client
	config              *CNIConfig
	helmChartInfoGetter *config.HelmChartGetter

	variableName string
	variablePath []string
}

var (
	_ commonhandlers.Named                   = &MultusCNI{}
	_ lifecycle.AfterControlPlaneInitialized = &MultusCNI{}
	_ lifecycle.BeforeClusterUpgrade         = &MultusCNI{}
)

func New(
	c ctrlclient.Client,
	cfg *CNIConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *MultusCNI {
	return &MultusCNI{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", v1alpha1.CNIVariableName},
	}
}

func (m *MultusCNI) Name() string {
	return "MultusCNI"
}

func (m *MultusCNI) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	m.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (m *MultusCNI) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	m.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (m *MultusCNI) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp *runtimehooksv1.CommonResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)

	// Get CNI configuration to detect primary CNI
	cniVar, err := variables.Get[v1alpha1.CNI](varMap, m.variableName, m.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info(
				"Skipping Multus CNI handler, cluster does not specify CNI addon deployment",
			)
			return
		}
		log.Error(
			err,
			"failed to read CNI provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CNI provider from cluster definition: %v",
				err,
			),
		)
		return
	}

	// Only deploy Multus if primary CNI is Cilium or Calico
	if cniVar.Provider != v1alpha1.CNIProviderCilium && cniVar.Provider != v1alpha1.CNIProviderCalico {
		log.V(5).Info(
			fmt.Sprintf(
				"Skipping Multus CNI handler, unsupported primary CNI provider: %q",
				cniVar.Provider,
			),
		)
		return
	}

	log.Info(
		fmt.Sprintf(
			"Deploying Multus CNI alongside primary CNI: %q",
			cniVar.Provider,
		),
	)

	// Wait for primary CNI to be ready before deploying Multus
	if err := m.waitForPrimaryCNI(ctx, cluster, cniVar.Provider, log); err != nil {
		log.Error(
			err,
			"failed waiting for primary CNI to be ready",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed waiting for primary CNI: %v",
				err,
			),
		)
		return
	}

	// Get Helm chart info
	helmChart, err := m.helmChartInfoGetter.For(ctx, log, config.Multus)
	if err != nil {
		log.Error(
			err,
			"failed to get configmap with helm settings",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to get configuration to create helm addon: %v",
				err,
			),
		)
		return
	}

	helmValuesSourceRefName := m.config.helmAddonConfig.defaultValuesTemplateConfigMapName
	targetNamespace := m.config.DefaultsNamespace()

	// Create HelmAddon strategy with value templater
	strategy := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			helmValuesSourceRefName,
			defaultMultusNamespace,
			defaultMultusReleaseName,
		),
		m.client,
		helmChart,
	).
		WithValueTemplater(func(cluster *clusterv1.Cluster, valuesTemplate string) (string, error) {
			return templateValues(cluster, cniVar.Provider)
		}).
		WithDefaultWaiter()

	if err := strategy.Apply(ctx, cluster, targetNamespace, log); err != nil {
		log.Error(
			err,
			"failed to apply Multus CNI",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to apply Multus CNI: %v",
				err,
			),
		)
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	log.Info("Successfully deployed Multus CNI")
}

func (m *MultusCNI) waitForPrimaryCNI(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	cniProvider string,
	log logr.Logger,
) error {
	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		m.client,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return fmt.Errorf("error creating remote cluster client: %w", err)
	}

	var daemonSetName string
	switch cniProvider {
	case v1alpha1.CNIProviderCilium:
		daemonSetName = "cilium"
	case v1alpha1.CNIProviderCalico:
		daemonSetName = "calico-node"
	default:
		return fmt.Errorf("unsupported CNI provider: %s", cniProvider)
	}

	log.Info(
		fmt.Sprintf(
			"Waiting for primary CNI DaemonSet %q to be ready",
			daemonSetName,
		),
	)

	// Wait for DaemonSet to be ready
	timeoutCtx, cancel := context.WithTimeout(ctx, maxWaitTime)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf(
				"timeout waiting for primary CNI DaemonSet %q to be ready",
				daemonSetName,
			)
		default:
			ds := &appsv1.DaemonSet{}
			err := remoteClient.Get(ctx, ctrlclient.ObjectKey{
				Namespace: metav1.NamespaceSystem,
				Name:      daemonSetName,
			}, ds)

			if err == nil && ds.Status.NumberReady > 0 &&
				ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
				log.Info(
					fmt.Sprintf(
						"Primary CNI DaemonSet %q is ready",
						daemonSetName,
					),
				)
				return nil
			}

			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("error checking DaemonSet: %w", err)
			}

			time.Sleep(waitInterval)
		}
	}
}
