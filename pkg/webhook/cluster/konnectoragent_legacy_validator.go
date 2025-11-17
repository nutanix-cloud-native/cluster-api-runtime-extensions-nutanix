// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// legacyHelmChartName is the chart name of the old helm release
	// that needs to be detected during upgrades.
	legacyHelmChartName = "nutanix-k8s-agent"
)

type konnectorAgentLegacyValidator struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewKonnectorAgentLegacyValidator(
	client ctrlclient.Client, decoder admission.Decoder,
) *konnectorAgentLegacyValidator {
	return &konnectorAgentLegacyValidator{
		client:  client,
		decoder: decoder,
	}
}

func (k *konnectorAgentLegacyValidator) Validator() admission.HandlerFunc {
	return k.validate
}

func (k *konnectorAgentLegacyValidator) validate(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	log := ctrl.LoggerFrom(ctx)

	// Only validate on UPDATE operations
	// Skip CREATE and DELETE operations
	if req.Operation != v1.Update {
		log.Info("Skipping validation for non-UPDATE operations", "operation", req.Operation)
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	err := k.decoder.Decode(req, cluster)
	if err != nil {
		log.Error(err, "Failed to decode cluster")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if cluster.Spec.Topology == nil {
		log.Info("Skipping validation for cluster without topology")
		return admission.Allowed("")
	}

	// Skip validation if the skip annotation is present
	if hasKonnectorAgentSkipAnnotation(cluster) {
		log.Info("Skipping validation for cluster with skip annotation")
		return admission.Allowed("")
	}

	// This check requires connecting to the workload cluster to list Helm releases.
	// Skip validation if infrastructure is not ready, as we cannot connect to the cluster yet.
	// This can happen during UPDATE operations early in cluster provisioning.
	if !cluster.Status.InfrastructureReady {
		log.Info("Skipping validation for cluster without infrastructure ready")
		return admission.Allowed("")
	}

	// Only check if konnector agent is enabled in the cluster configuration
	// Skip the check if the addon is not configured
	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)
	_, err = variables.Get[apivariables.NutanixKonnectorAgent](
		varMap,
		v1alpha1.ClusterConfigVariableName,
		[]string{"addons", v1alpha1.KonnectorAgentVariableName}...,
	)
	if err != nil {
		// If there's an error reading the variable, allow the operation to proceed
		log.Info("Failed to get konnector agent addon, allowing operation", "error", err)
		return admission.Allowed("")
	}

	// Get REST config for the cluster
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	restConfig, err := remote.RESTConfig(ctx, "", k.client, clusterKey)
	if err != nil {
		// If we can't reach the workload cluster API,
		// skip the check to avoid blocking valid operations unnecessarily.
		log.Info("Failed to get REST config, allowing operation", "error", err)
		return admission.Allowed("")
	}

	// List and filter legacy Helm releases in the cluster
	legacyReleases, err := k.listLegacyHelmReleases(restConfig)
	if err != nil {
		// If legacy releases cannot be listed,
		// skip the check to avoid blocking valid upgrade operations unnecessarily.
		log.Info("Failed to list legacy Helm releases, allowing operation", "error", err)
		return admission.Allowed("")
	}

	if len(legacyReleases) == 0 {
		// No legacy releases found - check passed
		log.Info("No legacy Helm releases found")
		return admission.Allowed("")
	}

	// Found legacy Helm releases - return error with instructions
	releaseDetails := make([]string, 0, len(legacyReleases))
	uninstallCommands := make([]string, 0, len(legacyReleases))
	forceUninstallCommands := make([]string, 0, len(legacyReleases))
	for _, rel := range legacyReleases {
		releaseDetails = append(releaseDetails, fmt.Sprintf("%s (namespace: %s)", rel.Name, rel.Namespace))
		uninstallCommands = append(
			uninstallCommands,
			fmt.Sprintf("helm uninstall %s -n %s --kubeconfig <kubeconfig-path>", rel.Name, rel.Namespace),
		)
		forceUninstallCommands = append(
			forceUninstallCommands,
			fmt.Sprintf("helm uninstall %s -n %s --no-hooks --kubeconfig <kubeconfig-path>", rel.Name, rel.Namespace),
		)
	}

	releaseInfo := fmt.Sprintf(
		"%d release(s) for chart %q: %v",
		len(legacyReleases),
		legacyHelmChartName,
		releaseDetails,
	)

	message := fmt.Sprintf(
		"\nCannot enable onboarding functionality as an addon: legacy installation(s) detected.\n\n"+
			"Found %s in the cluster.\n\n"+
			"ACTION REQUIRED: Uninstall the legacy Helm release(s) before proceeding to avoid conflicts.\n\n"+
			"To uninstall, run the following command(s):\n  %s\n\n"+
			"If the release is stuck or uninstall fails, use the force removal command:\n  %s\n\n"+
			"After removing the legacy release(s), re-run the operation.",
		releaseInfo,
		strings.Join(uninstallCommands, "\n  "),
		strings.Join(forceUninstallCommands, "\n  "),
	)

	return admission.Denied(message)
}

// listLegacyHelmReleases lists and filters Helm releases to find legacy releases
// with the specified chart name across all namespaces.
func (k *konnectorAgentLegacyValidator) listLegacyHelmReleases(
	restConfig *rest.Config,
) ([]*release.Release, error) {
	// Initialize Helm action configuration
	// Use empty namespace when AllNamespaces=true to allow listing from all namespaces
	actionConfig, err := k.initHelmActionConfig(restConfig, "")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Helm action config: %w", err)
	}

	// List Helm releases across all namespaces using Helm's List action.
	listAction := action.NewList(actionConfig)
	listAction.AllNamespaces = true
	// Include all release states to catch legacy releases in any state
	listAction.StateMask = action.ListDeployed | action.ListFailed | action.ListUninstalling |
		action.ListSuperseded | action.ListPendingInstall | action.ListPendingUpgrade |
		action.ListPendingRollback

	releases, err := listAction.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list Helm releases: %w", err)
	}

	// Filter releases by chart name to find legacy releases.
	var legacyReleases []*release.Release
	for _, rel := range releases {
		if rel.Chart != nil && rel.Chart.Name() == legacyHelmChartName {
			legacyReleases = append(legacyReleases, rel)
		}
	}
	return legacyReleases, nil
}

// initHelmActionConfig initializes a Helm action configuration for the given namespace.
func (k *konnectorAgentLegacyValidator) initHelmActionConfig(
	restConfig *rest.Config,
	namespace string,
) (*action.Configuration, error) {
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.WrapConfigFn = func(*rest.Config) *rest.Config {
		return restConfig
	}
	// Override namespace to allow access to all namespaces when AllNamespaces=true
	configFlags.Namespace = nil

	// Create a Helm action configuration
	actionConfig := new(action.Configuration)

	// Initialize the action configuration with the RESTClientGetter
	if err := actionConfig.Init(
		configFlags,
		namespace,
		"secret", // Helm storage driver (secrets)
		func(format string, v ...interface{}) {
			// Use a no-op logger for Helm debug messages
		},
	); err != nil {
		return nil, fmt.Errorf("failed to initialize Helm action config: %w", err)
	}

	return actionConfig, nil
}

func hasKonnectorAgentSkipAnnotation(cluster *clusterv1.Cluster) bool {
	if cluster.Annotations == nil {
		return false
	}
	val, ok := cluster.Annotations[v1alpha1.SkipKonnectorAgentLegacyDeploymentValidation]
	return ok && val == "true"
}
