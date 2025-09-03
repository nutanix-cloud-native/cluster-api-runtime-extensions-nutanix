// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/wait"
)

type CNIConfig struct {
	*options.GlobalOptions

	crsConfig       crsConfig
	helmAddonConfig helmAddonConfig
}

const (
	defaultCiliumReleaseName = "cilium"
	defaultCiliumNamespace   = metav1.NamespaceSystem
)

type helmAddonConfig struct {
	defaultValuesTemplateConfigMapName string
}

func (c *helmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-cilium-cni-helm-values-template",
		"default values ConfigMap name",
	)
}

func (c *CNIConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.crsConfig.AddFlags(prefix+".crs", flags)
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type CiliumCNI struct {
	client              ctrlclient.Client
	config              *CNIConfig
	helmChartInfoGetter *config.HelmChartGetter

	variableName string
	variablePath []string
}

var (
	_ commonhandlers.Named                   = &CiliumCNI{}
	_ lifecycle.AfterControlPlaneInitialized = &CiliumCNI{}
	_ lifecycle.BeforeClusterUpgrade         = &CiliumCNI{}
)

func New(
	c ctrlclient.Client,
	cfg *CNIConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *CiliumCNI {
	return &CiliumCNI{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", v1alpha1.CNIVariableName},
	}
}

func (c *CiliumCNI) Name() string {
	return "CiliumCNI"
}

func (c *CiliumCNI) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	c.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (c *CiliumCNI) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	c.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (c *CiliumCNI) apply(
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

	cniVar, err := variables.Get[v1alpha1.CNI](varMap, c.variableName, c.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info(
					"Skipping Cilium CNI handler, cluster does not specify request CNI addon deployment",
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
	if cniVar.Provider != v1alpha1.CNIProviderCilium {
		log.V(5).Info(
			fmt.Sprintf(
				"Skipping Cilium CNI handler, cluster does not specify %q as value of CNI provider variable",
				v1alpha1.CNIProviderCilium,
			),
		)
		return
	}

	targetNamespace := c.config.DefaultsNamespace()

	var strategy addons.Applier
	switch cniVar.Strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		strategy = crsStrategy{
			config: c.config.crsConfig,
			client: c.client,
		}
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := c.helmChartInfoGetter.For(ctx, log, config.Cilium)
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

		helmValuesSourceRefName := c.config.helmAddonConfig.defaultValuesTemplateConfigMapName
		if cniVar.Values != nil && cniVar.Values.SourceRef != nil {
			helmValuesSourceRefName = cniVar.Values.SourceRef.Name
			// Use cluster's namespace since Values.SourceRef is always a LocalObjectReference
			targetNamespace = cluster.Namespace

			err := handlersutils.EnsureClusterOwnerReferenceForObject(
				ctx,
				c.client,
				corev1.TypedLocalObjectReference{
					Kind: cniVar.Values.SourceRef.Kind,
					Name: cniVar.Values.SourceRef.Name,
				},
				cluster,
			)
			if err != nil {
				log.Error(
					err,
					"error updating Cluster's owner reference on Cilium helm values source object",
					"name",
					cniVar.Values.SourceRef.Name,
					"kind",
					cniVar.Values.SourceRef.Kind,
				)
				resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
				resp.SetMessage(
					fmt.Sprintf(
						"failed to set Cluster's owner reference on Cilium helm values source object: %v",
						err,
					),
				)
			}
		}

		strategy = addons.NewHelmAddonApplier(
			addons.NewHelmAddonConfig(
				helmValuesSourceRefName,
				defaultCiliumNamespace,
				defaultCiliumReleaseName,
			),
			c.client,
			helmChart,
		).
			WithDefaultWaiter()
	case "":
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage("strategy not specified for Cilium CNI addon")
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("unknown CNI addon deployment strategy %q", cniVar.Strategy))
		return
	}

	if err := runApply(ctx, c.client, cluster, strategy, targetNamespace, log); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func runApply(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
	strategy addons.Applier,
	targetNamespace string,
	log logr.Logger,
) error {
	if err := strategy.Apply(ctx, cluster, targetNamespace, log); err != nil {
		return err
	}

	// If skip kube-proxy is not set, return early.
	// Otherwise, wait for Cilium to be rolled out and then cleanup kube-proxy if installed.
	if !capiutils.ShouldSkipKubeProxy(cluster) {
		return nil
	}

	log.Info(
		fmt.Sprintf("Waiting for Cilium to be ready for cluster %s", ctrlclient.ObjectKeyFromObject(cluster)),
	)
	if err := waitForCiliumToBeReady(ctx, client, cluster); err != nil {
		return fmt.Errorf("failed to wait for Cilium to be ready: %w", err)
	}

	log.Info(
		fmt.Sprintf("Cleaning up kube-proxy for cluster %s", ctrlclient.ObjectKeyFromObject(cluster)),
	)
	if err := cleanupKubeProxy(ctx, client, cluster); err != nil {
		return fmt.Errorf("failed to cleanup kube-proxy: %w", err)
	}

	return nil
}

const (
	kubeProxyName      = "kube-proxy"
	kubeProxyNamespace = "kube-system"
)

func waitForCiliumToBeReady(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
) error {
	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		c,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return fmt.Errorf("error creating remote cluster client: %w", err)
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultCiliumReleaseName,
			Namespace: defaultCiliumNamespace,
		},
	}
	if err := wait.ForObject(
		ctx,
		wait.ForObjectInput[*appsv1.DaemonSet]{
			Reader: remoteClient,
			Target: ds.DeepCopy(),
			Check: func(_ context.Context, obj *appsv1.DaemonSet) (bool, error) {
				return obj.Status.NumberAvailable == obj.Status.DesiredNumberScheduled && obj.Status.NumberUnavailable == 0, nil
			},
			Interval: 1 * time.Second,
			Timeout:  30 * time.Second,
		},
	); err != nil {
		return fmt.Errorf(
			"failed to wait for DaemonSet %s to be Ready: %w",
			ctrlclient.ObjectKeyFromObject(ds),
			err,
		)
	}

	return nil
}

// cleanupKubeProxy cleans up kube-proxy DaemonSet and ConfigMap on the remote cluster when kube-proxy is disabled.
func cleanupKubeProxy(ctx context.Context, c ctrlclient.Client, cluster *clusterv1.Cluster) error {
	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		c,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return fmt.Errorf("error creating remote cluster client: %w", err)
	}

	objs := []ctrlclient.Object{
		&appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kubeProxyName,
				Namespace: kubeProxyNamespace,
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kubeProxyName,
				Namespace: kubeProxyNamespace,
			},
		},
	}
	for _, obj := range objs {
		if err := ctrlclient.IgnoreNotFound(remoteClient.Delete(ctx, obj)); err != nil {
			return fmt.Errorf("failed to delete %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return nil
}
