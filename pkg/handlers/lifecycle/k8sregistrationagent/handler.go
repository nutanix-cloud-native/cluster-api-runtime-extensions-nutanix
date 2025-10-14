// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package k8sregistrationagent

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
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
	defaultK8sAgentName          = "konnector-agent"
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
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter

	variableName string   // points to the global config variable
	variablePath []string // path of this variable on the global config variable
}

var (
	_ commonhandlers.Named                   = &DefaultK8sRegistrationAgent{}
	_ lifecycle.AfterControlPlaneInitialized = &DefaultK8sRegistrationAgent{}
	_ lifecycle.BeforeClusterUpgrade         = &DefaultK8sRegistrationAgent{}
	_ lifecycle.BeforeClusterDelete          = &DefaultK8sRegistrationAgent{}
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

func (n *DefaultK8sRegistrationAgent) BeforeClusterDelete(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterDeleteRequest,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	cluster := &req.Cluster
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
			log.Info(
				"Skipping K8s Registration Agent cleanup, addon not specified in cluster definition",
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
			return
		}
		log.Error(
			err,
			"failed to read K8s Registration Agent variable from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read K8s Registration Agent variable from cluster definition: %v",
				err,
			),
		)
		return
	}

	// Only handle HelmAddon strategy for cleanup
	switch k8sAgentVar.Strategy {
	case v1alpha1.AddonStrategyHelmAddon:
		// Check if cleanup is already in progress or completed
		cleanupStatus, err := n.checkCleanupStatus(ctx, cluster, log)
		if err != nil {
			log.Error(err, "Failed to check cleanup status")
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(err.Error())
			return
		}

		switch cleanupStatus {
		case "completed":
			log.Info("K8s Registration Agent cleanup already completed")
			resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
			return
		case "in-progress":
			log.Info("K8s Registration Agent cleanup in progress, requesting retry")
			resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
			resp.SetRetryAfterSeconds(10) // Retry after 10 seconds
			return
		case "not-started":
			log.Info("Starting K8s Registration Agent cleanup")
			// Proceed with cleanup below
		}

		err = n.deleteHelmChart(ctx, cluster, log)
		if err != nil {
			log.Error(err, "Failed to delete helm chart")
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(err.Error())
			return
		}

		// After initiating cleanup, request a retry to monitor completion
		log.Info("K8s Registration Agent cleanup initiated, will monitor progress")
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
		resp.SetRetryAfterSeconds(5) // Quick retry to start monitoring

	case v1alpha1.AddonStrategyClusterResourceSet:
		log.Info("ClusterResourceSet strategy does not require cleanup")
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	case "":
		log.Info("No strategy specified, skipping cleanup")
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	default:
		log.Info(
			"Unknown K8s Registration Agent strategy, skipping cleanup",
			"strategy", k8sAgentVar.Strategy,
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	}
}

func (n *DefaultK8sRegistrationAgent) deleteHelmChart(
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

	// Create HelmChartProxy with the same naming pattern used during creation
	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", defaultHelmReleaseName, clusterUUID),
			Namespace: cluster.Namespace,
		},
	}

	// First, try to gracefully trigger helm uninstall while cluster is still accessible
	log.Info("Initiating graceful deletion of K8s Registration Agent", "name", hcp.Name, "namespace", hcp.Namespace)

	// Get the current HCP to check if it exists and get its current state
	currentHCP := &caaphv1.HelmChartProxy{}
	err := n.client.Get(ctx, ctrlclient.ObjectKeyFromObject(hcp), currentHCP)
	if err != nil {
		if ctrlclient.IgnoreNotFound(err) == nil {
			log.Info("K8s Registration Agent HelmChartProxy already deleted", "name", hcp.Name)
			return nil
		}
		return fmt.Errorf("failed to get HelmChartProxy %q: %w", ctrlclient.ObjectKeyFromObject(hcp), err)
	}

	// Add a deletion timestamp annotation to help CAAPH prioritize this deletion
	// and set a shorter timeout to fail fast if cluster becomes unreachable
	if currentHCP.Annotations == nil {
		currentHCP.Annotations = make(map[string]string)
	}
	currentHCP.Annotations["cluster.x-k8s.io/delete-priority"] = "high"
	currentHCP.Annotations["cluster.x-k8s.io/delete-timeout"] = "60s"

	// Update the HCP with priority annotations before deletion
	if err := n.client.Update(ctx, currentHCP); err != nil {
		log.Info("Failed to update HCP annotations, proceeding with deletion", "error", err)
	}

	// Now delete the HelmChartProxy - CAAPH will handle the helm uninstall
	log.Info("Deleting K8s Registration Agent HelmChartProxy", "name", hcp.Name, "namespace", hcp.Namespace)
	if err := n.client.Delete(ctx, currentHCP); err != nil {
		if ctrlclient.IgnoreNotFound(err) == nil {
			log.Info("K8s Registration Agent HelmChartProxy already deleted", "name", hcp.Name)
			return nil
		}
		return fmt.Errorf(
			"failed to delete K8s Registration Agent HelmChartProxy %q: %w",
			ctrlclient.ObjectKeyFromObject(hcp),
			err,
		)
	}

	// Wait for CAAPH to complete the helm uninstall before allowing cluster deletion to proceed
	// This ensures graceful deletion order - helm uninstall completes before infrastructure teardown
	log.Info("Waiting for helm uninstall to complete before proceeding with cluster deletion", "name", hcp.Name)

	if err := n.waitForHelmUninstallCompletion(ctx, hcp, log); err != nil {
		log.Error(err, "Helm uninstall did not complete gracefully, proceeding with cluster deletion", "name", hcp.Name)
		// Don't return error here - we want cluster deletion to proceed even if helm uninstall times out
		// The important thing is we gave it a reasonable chance to complete
	} else {
		log.Info("Helm uninstall completed successfully", "name", hcp.Name)
	}

	return nil
}

// checkCleanupStatus checks the current status of K8s Registration Agent cleanup
// Returns: "completed", "in-progress", or "not-started"
func (n *DefaultK8sRegistrationAgent) checkCleanupStatus(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) (string, error) {
	clusterUUID, ok := cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
	if !ok {
		return "completed", nil // If no UUID, assume no agent was installed
	}

	// Check if HelmChartProxy exists
	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", defaultHelmReleaseName, clusterUUID),
			Namespace: cluster.Namespace,
		},
	}

	err := n.client.Get(ctx, ctrlclient.ObjectKeyFromObject(hcp), hcp)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("HelmChartProxy not found, cleanup completed", "name", hcp.Name)
			return "completed", nil
		}
		return "", fmt.Errorf("failed to get HelmChartProxy %q: %w", ctrlclient.ObjectKeyFromObject(hcp), err)
	}

	// HCP exists - check if it's being deleted
	if hcp.DeletionTimestamp != nil {
		log.Info("HelmChartProxy is being deleted, cleanup in progress", "name", hcp.Name)
		return "in-progress", nil
	}

	// HCP exists and is not being deleted
	log.Info("HelmChartProxy exists, cleanup not started", "name", hcp.Name)
	return "not-started", nil
}

// waitForHelmUninstallCompletion waits for CAAPH to complete the helm uninstall process
// before allowing cluster deletion to proceed. This ensures graceful deletion order.
func (n *DefaultK8sRegistrationAgent) waitForHelmUninstallCompletion(
	ctx context.Context,
	hcp *caaphv1.HelmChartProxy,
	log logr.Logger,
) error {
	// Create a context with timeout to avoid blocking cluster deletion indefinitely
	// 90 seconds should be enough for most helm uninstalls while still being reasonable
	waitCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	log.Info("Monitoring HelmChartProxy deletion progress", "name", hcp.Name)

	// First wait for the HelmChartProxy to be fully processed for deletion
	// This indicates CAAPH has acknowledged the deletion request
	err := wait.PollUntilContextTimeout(
		waitCtx,
		2*time.Second,
		30*time.Second,
		true,
		func(pollCtx context.Context) (bool, error) {
			currentHCP := &caaphv1.HelmChartProxy{}
			err := n.client.Get(pollCtx, ctrlclient.ObjectKeyFromObject(hcp), currentHCP)
			if err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("HelmChartProxy has been deleted", "name", hcp.Name)
					return true, nil
				}
				// If we can't reach the API server, the cluster might be shutting down
				// In this case, we should not block cluster deletion
				log.Info("Error checking HelmChartProxy status, cluster may be shutting down", "error", err)
				return true, nil
			}

			// Check if the HCP is in deletion phase
			if currentHCP.DeletionTimestamp != nil {
				log.Info("HelmChartProxy is being deleted, waiting for completion", "name", hcp.Name)
				return false, nil
			}

			// If HCP still exists without deletion timestamp, something might be wrong
			log.Info("HelmChartProxy still exists, waiting for deletion to start", "name", hcp.Name)
			return false, nil
		},
	)
	if err != nil {
		if wait.Interrupted(err) {
			return fmt.Errorf("timeout waiting for HelmChartProxy deletion to complete")
		}
		return fmt.Errorf("error waiting for HelmChartProxy deletion: %w", err)
	}

	// Additional wait to give CAAPH more time to complete the helm uninstall
	// even after the HCP is deleted. This accounts for any cleanup operations.
	log.Info("HelmChartProxy deleted, allowing additional time for helm uninstall completion")

	// Use a shorter additional wait to not delay cluster deletion too much
	additionalWaitCtx, additionalCancel := context.WithTimeout(ctx, 30*time.Second)
	defer additionalCancel()

	select {
	case <-additionalWaitCtx.Done():
		log.Info("Additional wait period completed, proceeding with cluster deletion")
	case <-time.After(10 * time.Second):
		log.Info("Reasonable wait time elapsed, proceeding with cluster deletion")
	}

	return nil
}
