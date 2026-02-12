// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	k8sclient "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	lifecycleconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/wait"
)

const (
	defaultCiliumValuesTemplateKey          = "values.yaml"
	defaultCiliumPreflightValuesTemplateKey = "preflight-values.yaml"
)

var (
	HelmReleaseNameHashLabel = "addons.cluster.x-k8s.io/helm-release-name-hash"
	ClusterNamespaceLabel    = clusterv1.ClusterNamespaceAnnotation
)

type HelmAddonConfig struct {
	defaultValuesTemplateConfigMapName string

	defaultHelmReleaseNamespace string
	defaultHelmReleaseName      string
}

func NewHelmAddonConfig(
	defaultValuesTemplateConfigMapName string,
	defaultHelmReleaseNamespace string,
	defaultHelmReleaseName string,
) *HelmAddonConfig {
	return &HelmAddonConfig{
		defaultValuesTemplateConfigMapName: defaultValuesTemplateConfigMapName,
		defaultHelmReleaseNamespace:        defaultHelmReleaseNamespace,
		defaultHelmReleaseName:             defaultHelmReleaseName,
	}
}

func (c *HelmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		c.defaultValuesTemplateConfigMapName,
		"default values ConfigMap name",
	)
}

type helmAddonApplier struct {
	config    *HelmAddonConfig
	client    ctrlclient.Client
	helmChart *lifecycleconfig.HelmChart
	opts      []applyOption
}

var (
	_ Applier = &helmAddonApplier{}
	_ Deleter = &helmAddonApplier{}
)

func NewHelmAddonApplier(
	config *HelmAddonConfig,
	client ctrlclient.Client,
	helmChart *lifecycleconfig.HelmChart,
) *helmAddonApplier {
	return &helmAddonApplier{
		config:    config,
		client:    client,
		helmChart: helmChart,
	}
}

type valueTemplaterFunc func(cluster *clusterv1.Cluster, valuesTemplate string) (string, error)

type waiterFunc func(ctx context.Context, client ctrlclient.Client, hcp *caaphv1.HelmChartProxy) error

type hooksFuncs struct {
	postApplyHookFuncs []postApplyHookFunc
}

type postApplyHookFunc func(
	ctx context.Context,
	client ctrlclient.Client,
	remoteClient ctrlclient.Client,
	cluster *clusterv1.Cluster,
	hcp *caaphv1.HelmChartProxy,
) error

type applyOptions struct {
	valueTemplater     valueTemplaterFunc
	targetCluster      *clusterv1.Cluster
	helmReleaseName    string
	shouldRunPreflight bool
	waiter             waiterFunc
	hooks              hooksFuncs
}

type applyOption func(*applyOptions)

func (a *helmAddonApplier) WithValueTemplater(
	valueTemplater valueTemplaterFunc,
) *helmAddonApplier {
	a.opts = append(a.opts, func(o *applyOptions) {
		o.valueTemplater = valueTemplater
	})

	return a
}

func (a *helmAddonApplier) WithTargetCluster(cluster *clusterv1.Cluster) *helmAddonApplier {
	a.opts = append(a.opts, func(o *applyOptions) {
		o.targetCluster = cluster
	})

	return a
}

func (a *helmAddonApplier) WithHelmReleaseName(name string) *helmAddonApplier {
	a.opts = append(a.opts, func(o *applyOptions) {
		o.helmReleaseName = name
	})

	return a
}

func (a *helmAddonApplier) WithPreflightEnabled() *helmAddonApplier {
	a.opts = append(a.opts, func(o *applyOptions) {
		o.shouldRunPreflight = true
	})

	return a
}

func (a *helmAddonApplier) WithDefaultWaiter() *helmAddonApplier {
	a.opts = append(a.opts, func(o *applyOptions) {
		o.waiter = waitToBeReady
	})

	return a
}

func (a *helmAddonApplier) WithPostApplyHook(postApplyHookFuncs ...postApplyHookFunc) *helmAddonApplier {
	a.opts = append(a.opts, func(o *applyOptions) {
		o.hooks.postApplyHookFuncs = postApplyHookFuncs
	})

	return a
}

func (a *helmAddonApplier) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	defaultsNamespace string,
	log logr.Logger,
) error {
	clusterUUID, ok := cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
	if !ok {
		return fmt.Errorf(
			"cluster UUID not found in cluster annotations - missing key %s",
			v1alpha1.ClusterUUIDAnnotationKey,
		)
	}

	applyOpts := &applyOptions{}
	for _, opt := range a.opts {
		opt(applyOpts)
	}

	configMapTemplateKey := defaultCiliumValuesTemplateKey
	// override the config map key to retrieve preflight key
	if applyOpts.shouldRunPreflight {
		configMapTemplateKey = defaultCiliumPreflightValuesTemplateKey
	}

	log.Info("Retrieving installation values template for cluster")
	values, err := handlersutils.RetrieveValuesTemplate(
		ctx,
		a.client,
		a.config.defaultValuesTemplateConfigMapName,
		configMapTemplateKey,
		defaultsNamespace,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve installation values template for cluster: %w",
			err,
		)
	}

	if applyOpts.valueTemplater != nil {
		values, err = applyOpts.valueTemplater(cluster, values)
		if err != nil {
			return fmt.Errorf("failed to template Helm values: %w", err)
		}
	}

	targetCluster := cluster
	if applyOpts.targetCluster != nil {
		targetCluster = applyOpts.targetCluster
	}

	helmReleaseName := a.config.defaultHelmReleaseName
	if applyOpts.helmReleaseName != "" {
		helmReleaseName = applyOpts.helmReleaseName
	}

	chartProxy := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: targetCluster.Namespace,
			Name:      fmt.Sprintf("%s-%s", a.config.defaultHelmReleaseName, clusterUUID),
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   a.helmChart.Repository,
			ChartName: a.helmChart.Name,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: targetCluster.Name},
			},
			ReleaseNamespace: a.config.defaultHelmReleaseNamespace,
			ReleaseName:      helmReleaseName,
			Version:          a.helmChart.Version,
			ValuesTemplate:   values,
		},
	}

	handlersutils.SetTLSConfigForHelmChartProxyIfNeeded(chartProxy)
	if err = controllerutil.SetOwnerReference(targetCluster, chartProxy, a.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on HelmChartProxy %q: %w",
			chartProxy.Name,
			err,
		)
	}

	if err = k8sclient.ServerSideApply(ctx, a.client, chartProxy, k8sclient.ForceOwnership); err != nil {
		return fmt.Errorf("failed to apply HelmChartProxy %q: %w", chartProxy.Name, err)
	}

	// Run post apply hooks that need to run after the HelmChartProxy is applied.
	// These hooks may be useful during upgrades to perform additional actions
	// either on the management or remote cluster to unblock a Helm upgrade.
	if len(applyOpts.hooks.postApplyHookFuncs) > 0 {
		remoteClient, err := remote.NewClusterClient(ctx, "", a.client, ctrlclient.ObjectKeyFromObject(cluster))
		if err != nil {
			return fmt.Errorf("error creating remote cluster client: %w", err)
		}
		for _, hook := range applyOpts.hooks.postApplyHookFuncs {
			if err = hook(ctx, a.client, remoteClient, cluster, chartProxy); err != nil {
				return fmt.Errorf("failed to run post apply hook: %w", err)
			}
		}
	}

	if applyOpts.waiter != nil {
		return applyOpts.waiter(ctx, a.client, chartProxy)
	}

	return nil
}

// Delete removes the HelmChartProxy (HCP) for the given cluster, which triggers
// the Helm release uninstall on the target cluster.
func (a *helmAddonApplier) Delete(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	clusterUUID, ok := cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
	if !ok {
		return fmt.Errorf(
			"cluster UUID not found in cluster annotations - missing key %s",
			v1alpha1.ClusterUUIDAnnotationKey,
		)
	}

	applyOpts := &applyOptions{}
	for _, opt := range a.opts {
		opt(applyOpts)
	}

	targetCluster := cluster
	if applyOpts.targetCluster != nil {
		targetCluster = applyOpts.targetCluster
	}

	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", a.config.defaultHelmReleaseName, clusterUUID),
			Namespace: targetCluster.Namespace,
		},
	}

	if err := ctrlclient.IgnoreNotFound(a.client.Delete(ctx, hcp)); err != nil {
		return fmt.Errorf(
			"failed to delete HelmChartProxy %q: %w",
			ctrlclient.ObjectKeyFromObject(hcp),
			err,
		)
	}

	return nil
}

func waitToBeReady(
	ctx context.Context,
	client ctrlclient.Client,
	hcp *caaphv1.HelmChartProxy,
) error {
	if err := wait.ForObject(
		ctx,
		wait.ForObjectInput[*caaphv1.HelmChartProxy]{
			Reader: client,
			Target: hcp.DeepCopy(),
			Check: func(_ context.Context, obj *caaphv1.HelmChartProxy) (bool, error) {
				if obj.Generation != obj.Status.ObservedGeneration {
					return false, nil
				}
				return conditions.IsTrue(obj, clusterv1.ReadyCondition), nil
			},
			Interval: 5 * time.Second,
			Timeout:  30 * time.Second,
		},
	); err != nil {
		return fmt.Errorf(
			"failed to wait for addon %s to deploy: %w",
			ctrlclient.ObjectKeyFromObject(hcp),
			err,
		)
	}

	return nil
}
