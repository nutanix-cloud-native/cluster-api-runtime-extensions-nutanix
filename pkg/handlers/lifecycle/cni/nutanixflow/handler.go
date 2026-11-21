// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanixflow

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultNutanixFlowReleaseName = "flow-cni"
	defaultNutanixFlowNamespace   = "kube-system"

	//nolint:gosec // Does not contain hard coded credentials.
	defaultImagePullSecretName = "flow-cni-image-pull-secret"
)

// flowNamespaces are where Flow CNI 1.1.0 deploys components. They must exist
// even when imagePullCredentials are omitted (registry mirror / air-gapped),
// because Helm installs workloads into these namespaces. When credentials are
// provided, the same namespaces also receive the copied imagePullSecret.
var flowNamespaces = []string{
	"flow-cni-system",
	"flow-cns-system",
	"ovn-kubernetes",
}

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
		"default-nutanix-flow-cni-helm-values-template",
		"default values ConfigMap name",
	)
}

func (c *CNIConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type NutanixFlowCNI struct {
	client              ctrlclient.Client
	config              *CNIConfig
	helmChartInfoGetter *config.HelmChartGetter

	variableName string
	variablePath []string
}

var (
	_ commonhandlers.Named                   = &NutanixFlowCNI{}
	_ lifecycle.AfterControlPlaneInitialized = &NutanixFlowCNI{}
	_ lifecycle.BeforeClusterUpgrade         = &NutanixFlowCNI{}
)

func New(
	c ctrlclient.Client,
	cfg *CNIConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *NutanixFlowCNI {
	return &NutanixFlowCNI{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", v1alpha1.CNIVariableName},
	}
}

func (c *NutanixFlowCNI) Name() string {
	return "NutanixFlowCNI"
}

func (c *NutanixFlowCNI) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	cluster := &req.Cluster
	commonResponse := &runtimehooksv1.CommonResponse{}
	c.apply(ctx, cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (c *NutanixFlowCNI) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	cluster := &req.Cluster
	commonResponse := &runtimehooksv1.CommonResponse{}
	c.apply(ctx, cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (c *NutanixFlowCNI) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp *runtimehooksv1.CommonResponse,
) {
	start := time.Now()
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
		"handler",
		c.Name(),
	)

	if cluster.Spec.InfrastructureRef.Kind != "NutanixCluster" {
		log.V(5).Info(
			"Skipping Nutanix Flow CNI handler, cluster infrastructure is not NutanixCluster",
		)
		return
	}

	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)

	cniVar, err := variables.Get[v1alpha1.CNI](varMap, c.variableName, c.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info(
				"Skipping Nutanix Flow CNI handler, cluster does not specify CNI addon deployment",
			)
			return
		}
		log.Error(err, "failed to read CNI provider from cluster definition")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CNI provider from cluster definition: %v", err),
		)
		return
	}
	if cniVar.Provider != v1alpha1.CNIProviderFlow {
		log.V(5).Info(
			fmt.Sprintf(
				"Skipping Nutanix Flow CNI handler, cluster does not specify %q as value of CNI provider variable",
				v1alpha1.CNIProviderFlow,
			),
		)
		return
	}

	hasImagePullCredentials := cniVar.ImagePullCredentials != nil &&
		cniVar.ImagePullCredentials.SecretRef.Name != ""
	sourceImagePullSecret := ""
	if hasImagePullCredentials {
		sourceImagePullSecret = cniVar.ImagePullCredentials.SecretRef.Name
	}
	log.Info("applying Nutanix Flow CNI",
		"provider", cniVar.Provider,
		"strategy", cniVar.Strategy,
		"hasImagePullCredentials", hasImagePullCredentials,
		"imagePullSecret", sourceImagePullSecret,
	)

	if cniVar.Strategy != v1alpha1.AddonStrategyHelmAddon {
		log.Error(nil, fmt.Sprintf(
			"Nutanix Flow CNI only supports %q strategy, got %q",
			v1alpha1.AddonStrategyHelmAddon,
			cniVar.Strategy,
		))
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf(
			"Nutanix Flow CNI only supports %q addon strategy",
			v1alpha1.AddonStrategyHelmAddon,
		))
		return
	}

	helmChart, err := c.helmChartInfoGetter.For(ctx, log, config.NutanixFlowCNI)
	if err != nil {
		log.Error(err, "failed to get configmap with helm settings")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to get configuration to create helm addon: %v", err),
		)
		return
	}
	log.Info("resolved Flow CNI helm chart",
		"chartName", helmChart.Name,
		"chartVersion", helmChart.Version,
		"repositoryURL", helmChart.Repository,
	)

	// Always create Flow namespaces up front. Previously they were only created
	// as a side effect of copying imagePullCredentials; with a registry mirror
	// Konvoy omits those credentials, so namespaces never appeared and Helm
	// could not install into them.
	remoteClientStart := time.Now()
	log.Info("creating remote cluster client for Flow CNI namespace ensure")
	remoteClient, err := remote.NewClusterClient(ctx, "", c.client, clusterKey)
	if err != nil {
		log.Error(err, "error creating client for remote cluster",
			"duration", time.Since(remoteClientStart).String(),
			"ctxErr", ctx.Err(),
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("error creating client for remote cluster: %v", err))
		return
	}
	log.Info("created remote cluster client for Flow CNI",
		"duration", time.Since(remoteClientStart).String(),
	)

	for _, ns := range flowNamespaces {
		nsStart := time.Now()
		log.Info("ensuring Flow CNI namespace on remote cluster", "namespace", ns)
		if nsErr := handlersutils.EnsureNamespaceWithName(ctx, remoteClient, ns); nsErr != nil {
			log.Error(nsErr, "error creating Flow CNI namespace on remote cluster",
				"namespace", ns,
				"duration", time.Since(nsStart).String(),
				"ctxErr", ctx.Err(),
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(fmt.Sprintf(
				"failed to create Flow CNI namespace %q on remote cluster: %v",
				ns,
				nsErr,
			))
			return
		}
		log.Info("ensured Flow CNI namespace on remote cluster",
			"namespace", ns,
			"duration", time.Since(nsStart).String(),
		)
	}

	var imagePullSecretName string
	if hasImagePullCredentials {
		secretName := cniVar.ImagePullCredentials.SecretRef.Name
		log.Info("copying Flow CNI image pull credentials to remote namespaces",
			"secret", secretName,
			"namespaces", flowNamespaces,
		)
		err := handlersutils.EnsureClusterOwnerReferenceForObject(
			ctx,
			c.client,
			corev1.TypedLocalObjectReference{
				Kind: "Secret",
				Name: secretName,
			},
			cluster,
		)
		if err != nil {
			log.Error(
				err,
				"error updating Cluster's owner reference on Flow CNI image pull Secret",
				"name", secretName,
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(fmt.Sprintf(
				"failed to set Cluster's owner reference on Flow CNI image pull Secret: %v",
				err,
			))
			return
		}

		for _, ns := range flowNamespaces {
			copyStart := time.Now()
			key := ctrlclient.ObjectKey{
				Name:      defaultImagePullSecretName,
				Namespace: ns,
			}
			err = handlersutils.CopySecretToRemoteCluster(
				ctx,
				c.client,
				secretName,
				key,
				cluster,
			)
			if err != nil {
				log.Error(
					err,
					"error copying image pull Secret for Flow CNI to remote cluster",
					"namespace", ns,
					"sourceSecret", secretName,
					"destinationSecret", defaultImagePullSecretName,
					"duration", time.Since(copyStart).String(),
					"ctxErr", ctx.Err(),
				)
				resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
				resp.SetMessage(fmt.Sprintf(
					"failed to copy image pull Secret for Flow CNI to namespace %q on remote cluster: %v",
					ns,
					err,
				))
				return
			}
			log.Info("copied Flow CNI image pull Secret to remote cluster",
				"namespace", ns,
				"destinationSecret", defaultImagePullSecretName,
				"duration", time.Since(copyStart).String(),
			)
		}

		imagePullSecretName = defaultImagePullSecretName
	} else {
		log.Info("skipping Flow CNI image pull Secret copy; no imagePullCredentials configured")
	}

	targetNamespace := c.config.DefaultsNamespace()

	helmValuesSourceRefName := c.config.helmAddonConfig.defaultValuesTemplateConfigMapName
	if cniVar.Values != nil && cniVar.Values.SourceRef != nil {
		helmValuesSourceRefName = cniVar.Values.SourceRef.Name
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
				"error updating Cluster's owner reference on Flow CNI helm values source object",
				"name", cniVar.Values.SourceRef.Name,
				"kind", cniVar.Values.SourceRef.Kind,
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(fmt.Sprintf(
				"failed to set Cluster's owner reference on Flow CNI helm values source object: %v",
				err,
			))
			return
		}
	}

	log.Info("applying Flow CNI HelmChartProxy",
		"releaseName", defaultNutanixFlowReleaseName,
		"releaseNamespace", defaultNutanixFlowNamespace,
		"valuesConfigMap", helmValuesSourceRefName,
		"valuesConfigMapNamespace", targetNamespace,
		"imagePullSecretName", imagePullSecretName,
	)

	strategy := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			helmValuesSourceRefName,
			defaultNutanixFlowNamespace,
			defaultNutanixFlowReleaseName,
		),
		c.client,
		helmChart,
	).
		WithValueTemplater(func(cluster *clusterv1.Cluster, text string) (string, error) {
			return templateValues(cluster, text, imagePullSecretName)
		}).
		WithDefaultWaiter().
		WithTakeOwnership()

	helmStart := time.Now()
	if err := strategy.Apply(ctx, cluster, targetNamespace, log); err != nil {
		log.Error(err, "failed to apply Flow CNI helm addon",
			"duration", time.Since(helmStart).String(),
			"totalDuration", time.Since(start).String(),
			"ctxErr", ctx.Err(),
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	log.Info("successfully applied Nutanix Flow CNI",
		"helmDuration", time.Since(helmStart).String(),
		"totalDuration", time.Since(start).String(),
	)
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
