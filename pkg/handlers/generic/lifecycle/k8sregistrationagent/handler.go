// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package k8sregistrationagent

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultHelmReleaseName       = "k8s-registration-agent"
	defaultHelmReleaseNamespace  = "ntnx-system"
	defaultK8sAgentName          = "nutanix-agent"
	defaultCredentialsSecretName = defaultK8sAgentName
)

type ControllerConfig struct {
	*options.GlobalOptions
	helmAddonConfig *addons.HelmAddonConfig
}

func NewControllerConfig(globalOptions *options.GlobalOptions) *ControllerConfig {
	return &ControllerConfig{
		GlobalOptions: globalOptions,
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-k8s-registrationagent-helm-values-template",
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
	}
}

func (c *ControllerConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type DefaultK8sRegistrationAgent struct {
	client              ctrlclient.Client
	config              *ControllerConfig
	helmChartInfoGetter *config.HelmChartGetter

	variableName string   // points to the global config variable
	variablePath []string // path of this variable on the global config variable
}

var (
	_ commonhandlers.Named                   = &DefaultK8sRegistrationAgent{}
	_ lifecycle.AfterControlPlaneInitialized = &DefaultK8sRegistrationAgent{}
	_ lifecycle.BeforeClusterUpgrade         = &DefaultK8sRegistrationAgent{}
)

func New(
	c ctrlclient.Client,
	cfg *ControllerConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *DefaultK8sRegistrationAgent {
	return &DefaultK8sRegistrationAgent{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", v1alpha1.K8sRegistrationAgentVariableName},
	}
}

func (n *DefaultK8sRegistrationAgent) Name() string {
	return "K8sRegistrationAgentHandler"
}

func (n *DefaultK8sRegistrationAgent) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultK8sRegistrationAgent) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultK8sRegistrationAgent) apply(
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
	k8sAgentVar, err := variables.Get[apivariables.NutanixK8sRegistrationAgent](
		varMap,
		n.variableName,
		n.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.
				Info(
					"Skipping K8s Registration Agent handler," +
						"cluster does not specify request K8s Registration Agent addon deployment",
				)
			return
		}
		log.Error(
			err,
			"failed to read K8s Registration Agent variable from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read K8s Registration agent variable from cluster definition: %v",
				err,
			),
		)
		return
	}

	// Ensure pc credentials are provided
	if k8sAgentVar.Credentials == nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage("name of the Secret containing PC credentials must be set")
		return
	}

	// It's possible to have the credentials Secret be created by the Helm chart.
	// However, that would leave the credentials visible in the HelmChartProxy.
	// Instead, we'll create the Secret on the remote cluster and reference it in the Helm values.
	if k8sAgentVar.Credentials != nil {
		err := handlersutils.EnsureClusterOwnerReferenceForObject(
			ctx,
			n.client,
			corev1.TypedLocalObjectReference{
				Kind: "Secret",
				Name: k8sAgentVar.Credentials.SecretRef.Name,
			},
			cluster,
		)
		if err != nil {
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(
				fmt.Sprintf("error updating owner references on Nutanix k8s agent source Secret: %v",
					err,
				),
			)
			return
		}
		key := ctrlclient.ObjectKey{
			Name:      defaultCredentialsSecretName,
			Namespace: defaultHelmReleaseNamespace,
		}
		err = handlersutils.CopySecretToRemoteCluster(
			ctx,
			n.client,
			k8sAgentVar.Credentials.SecretRef.Name,
			key,
			cluster,
		)
		if err != nil {
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(
				fmt.Sprintf("error creating Nutanix k8s agent Credentials Secret on the remote cluster: %v",
					err,
				),
			)
			return
		}
	}

	var strategy addons.Applier
	switch k8sAgentVar.Strategy {
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := n.helmChartInfoGetter.For(ctx, log, config.K8sRegistrationAgent)
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
		clusterConfigVar, err := variables.Get[apivariables.ClusterConfigSpec](
			varMap,
			v1alpha1.ClusterConfigVariableName,
		)
		if err != nil {
			log.Error(
				err,
				"failed to read clusterConfig variable from cluster definition",
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(
				fmt.Sprintf("failed to read clusterConfig variable from cluster definition: %v",
					err,
				),
			)
			return
		}
		strategy = addons.NewHelmAddonApplier(
			n.config.helmAddonConfig,
			n.client,
			helmChart,
		).WithValueTemplater(templateValuesFunc(clusterConfigVar.Nutanix, cluster))
	case v1alpha1.AddonStrategyClusterResourceSet:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"strategy %q not provided for K8s Registration Agent", v1alpha1.AddonStrategyClusterResourceSet,
			),
		)
		return
	case "":
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage("strategy not provided for K8s Registration Agent")
		return
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("unknown K8s registration agent addon deployment strategy %q", k8sAgentVar.Strategy),
		)
		return
	}

	if err := strategy.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		log.Error(err, "Helm strategy Apply failed")
		err = fmt.Errorf("failed to apply K8s Registration Agent addon: %w", err)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func templateValuesFunc(
	nutanixConfig *v1alpha1.NutanixSpec, cluster *clusterv1.Cluster,
) func(*clusterv1.Cluster, string) (string, error) {
	return func(_ *clusterv1.Cluster, valuesTemplate string) (string, error) {
		joinQuoted := template.FuncMap{
			"joinQuoted": func(items []string) string {
				for i, item := range items {
					items[i] = fmt.Sprintf("%q", item)
				}
				return strings.Join(items, ", ")
			},
		}
		helmValuesTemplate, err := template.New("").Funcs(joinQuoted).Parse(valuesTemplate)
		if err != nil {
			return "", fmt.Errorf("failed to parse Helm values template: %w", err)
		}

		type input struct {
			AgentName            string
			PrismCentralHost     string
			PrismCentralPort     uint16
			PrismCentralInsecure bool
			ClusterName          string
		}

		address, port, err := nutanixConfig.PrismCentralEndpoint.ParseURL()
		if err != nil {
			return "", err
		}
		templateInput := input{
			AgentName:            defaultK8sAgentName,
			PrismCentralHost:     address,
			PrismCentralPort:     port,
			PrismCentralInsecure: nutanixConfig.PrismCentralEndpoint.Insecure,
			ClusterName:          cluster.Name,
		}

		var b bytes.Buffer
		err = helmValuesTemplate.Execute(&b, templateInput)
		if err != nil {
			return "", fmt.Errorf("failed setting PrismCentral configuration in template: %w", err)
		}

		return b.String(), nil
	}
}
