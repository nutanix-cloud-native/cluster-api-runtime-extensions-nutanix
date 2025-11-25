// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
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

	// agentConfigMapName is the name of the ConfigMap that is deployed by both the legacy and new konnector agent.
	agentConfigMapName = "ntnx-cluster-configmap"

	// konnectorAgentReleaseName is the name of the release for the new konnector agent.
	konnectorAgentReleaseName = "konnector-agent"
	// konnectorAgentReleaseNamespace is the namespace of the release for the new konnector agent.
	konnectorAgentReleaseNamespace = "ntnx-system"

	releaseNameAnnotation      = "meta.helm.sh/release-name"
	releaseNamespaceAnnotation = "meta.helm.sh/release-namespace"
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
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	err := k.decoder.Decode(req, cluster)
	if err != nil {
		log.Error(err, "Failed to decode cluster")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if cluster.Spec.Topology == nil {
		return admission.Allowed("")
	}

	// Skip validation if the skip annotation is present
	if hasKonnectorAgentSkipAnnotation(cluster) {
		return admission.Allowed("")
	}

	// This check requires connecting to the workload cluster to list Helm releases.
	// Skip validation if infrastructure is not ready, as we cannot connect to the cluster yet.
	// This can happen during UPDATE operations early in cluster provisioning.
	if !cluster.Status.InfrastructureReady {
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
		return admission.Allowed("")
	}

	// Get remote client for the cluster.
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	remoteClient, err := remote.NewClusterClient(ctx, "", k.client, clusterKey)
	if err != nil {
		// If we can't reach the workload cluster API,
		// skip the check to avoid blocking valid operations unnecessarily.
		log.Info("Failed to get remote client, allowing operation for cluster", "error", err)
		return admission.Allowed("")
	}

	legacyReleases, err := findLegacyReleases(ctx, remoteClient)
	if err != nil {
		// If legacy releases cannot be listed,
		// skip the check to avoid blocking valid upgrade operations unnecessarily.
		log.Info("Failed to list legacy releases, allowing operation for cluster", "error", err)
		return admission.Allowed("")
	}

	if len(legacyReleases) == 0 {
		// No legacy releases found - check passed
		return admission.Allowed("")
	}

	// Found legacy Helm releases - return error with instructions
	releaseDetails := make([]string, 0, len(legacyReleases))
	uninstallCommands := make([]string, 0, len(legacyReleases))
	forceUninstallCommands := make([]string, 0, len(legacyReleases))
	for _, rel := range legacyReleases {
		releaseDetails = append(releaseDetails, fmt.Sprintf("%s/%s", rel.Namespace, rel.Name))
		uninstallCommands = append(
			uninstallCommands,
			fmt.Sprintf("helm uninstall %s --namespace %s --kubeconfig <kubeconfig-path>", rel.Name, rel.Namespace),
		)
		forceUninstallCommands = append(
			forceUninstallCommands,
			fmt.Sprintf(
				"helm uninstall %s --namespace %s --no-hooks --kubeconfig <kubeconfig-path>",
				rel.Name,
				rel.Namespace,
			),
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

type LegacyRelease struct {
	Name      string
	Namespace string
}

// findLegacyReleases finds the legacy Kubernetes Agent releases.
// Both the legacy and new konnector agent deploy a ConfigMap with the name ntnx-cluster-configmap.
// It finds the statically named ConfigMap and compares the annotations to the expected values.
// If the annotations are not empty and do not equal the release name and namespace of the new konnector agent,
// it assumes it was installed manually.
//
//	annotations:
//	  meta.helm.sh/release-name: konnector-agent
//	  meta.helm.sh/release-namespace: ntnx-system
func findLegacyReleases(ctx context.Context, client ctrlclient.Client) ([]LegacyRelease, error) {
	configMaps := &corev1.ConfigMapList{}
	// List in all Namespaces, but use field-selector to filter by name server-side.
	// In the worst case, this query returns N ConfigMaps, where N is the number of namespaces.
	// In practice, we expect it to return 0 or 1 ConfigMaps.
	listOptions := &ctrlclient.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{
			"metadata.name": agentConfigMapName,
		}),
		Namespace: metav1.NamespaceAll,
	}
	err := client.List(ctx, configMaps, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get config map: %w", err)
	}

	legacyReleases := make([]LegacyRelease, 0)
	for i := range configMaps.Items {
		configMap := &configMaps.Items[i]
		if configMap.Annotations == nil {
			continue
	}
	
	releaseNameAnnotationPresent, _ := configMap.Annotations[releaseNameAnnotation]
releaseNamespaceAnnotationPresent, _ := configMap.Annotations[releaseNamespaceAnnotation]

if !releaseNameAnnotationPresent || !releaseNamespaceAnnotationPresent {
   continue
}
		if configMap.Annotations[releaseNameAnnotation] == konnectorAgentReleaseName &&
			configMap.Annotations[releaseNamespaceAnnotation] == konnectorAgentReleaseNamespace {
			continue
		}
		// The annotations are not empty and do not equal the expected values, assume it was manually installed.
		legacyReleases = append(legacyReleases, LegacyRelease{
			Name:      configMap.Annotations[releaseNameAnnotation],
			Namespace: configMap.Annotations[releaseNamespaceAnnotation],
		})
	}

	return legacyReleases, nil
}

func hasKonnectorAgentSkipAnnotation(cluster *clusterv1.Cluster) bool {
	if cluster.Annotations == nil {
		return false
	}
	val, ok := cluster.Annotations[v1alpha1.SkipKonnectorAgentLegacyDeploymentValidation]
	return ok && val == "true"
}
