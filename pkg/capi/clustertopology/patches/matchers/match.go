// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// This code re-implements private matcher for CAPI inline patch selector.
// See: https://github.com/kubernetes-sigs/cluster-api/blob/46412f0a4ea65d8f02478d2ad09ce12925485f56/api/v1beta1/clusterclass_types.go#L509-L523
// See: https://github.com/kubernetes-sigs/cluster-api/blob/46412f0a4ea65d8f02478d2ad09ce12925485f56/internal/controllers/topology/cluster/patches/inline/json_patch_generator.go#L125
//
//nolint:lll // Long URLs in comments above. Adding nolint:lll here because it doesn't work in comment lines. See: https://github.com/golangci/golangci-lint/issues/3983
package matchers

import (
	"strconv"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
)

func MatchesSelector(
	selector clusterv1.PatchSelector,
	obj runtime.Object,
	holderRef *runtimehooksv1.HolderReference,
	templateVariables map[string]apiextensionsv1.JSON,
) bool {
	if !MatchesGVK(selector.APIVersion, selector.Kind, obj) {
		return false
	}

	if selector.MatchResources.InfrastructureCluster && MatchesInfrastructure(holderRef) {
		return true
	}

	if selector.MatchResources.ControlPlane && MatchesControlPlane(holderRef) {
		return true
	}

	if selector.MatchResources.MachineDeploymentClass != nil &&
		MatchesMachineDeploymentClass(
			holderRef,
			selector.MatchResources.MachineDeploymentClass.Names,
			templateVariables,
		) {
		return true
	}

	return false
}

// MatchesGVK checks if the apiVersion and kind are matching.
func MatchesGVK(apiVersion, kind string, obj runtime.Object) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()

	return gvk.GroupVersion().String() == apiVersion && gvk.Kind == kind
}

// MatchesInfrastructure checks if the request is for an InfrastructureCluster.
// Cluster.spec.infrastructureRef holds the InfrastructureCluster.
func MatchesInfrastructure(holderRef *runtimehooksv1.HolderReference) bool {
	return holderRef.Kind == "Cluster" && holderRef.FieldPath == "spec.infrastructureRef"
}

// MatchesControlPlane checks if the request is for a ControlPlane or the InfrastructureMachineTemplate of a
// ControlPlane.
func MatchesControlPlane(holderRef *runtimehooksv1.HolderReference) bool {
	// Cluster.spec.controlPlaneRef holds the ControlPlane.
	if holderRef.Kind == "Cluster" && holderRef.FieldPath == "spec.controlPlaneRef" {
		return true
	}
	// *.spec.machineTemplate.infrastructureRef holds the InfrastructureMachineTemplate of a ControlPlane.
	// Note: this field path is only used in this context.
	// NOTE(mh): https://github.com/kubernetes-sigs/cluster-api/blob/main/internal/contract/controlplane.go#L281-L286
	if holderRef.FieldPath == "spec.machineTemplate.infrastructureRef" {
		return true
	}

	return false
}

// MatchesMachineDeploymentClass checks if the request is for a BootstrapConfigTemplate or an
// InfrastructureMachineTemplate of one of the configured MachineDeploymentClasses.
func MatchesMachineDeploymentClass(
	holderRef *runtimehooksv1.HolderReference,
	names []string,
	templateVariables map[string]apiextensionsv1.JSON,
) bool {
	if holderRef.Kind == "MachineDeployment" &&
		(holderRef.FieldPath == "spec.template.spec.bootstrap.configRef" ||
			holderRef.FieldPath == "spec.template.spec.infrastructureRef") {
		// Read the builtin.machineDeployment.class variable.
		templateMDClassJSON, found, err := topologymutation.GetVariable(
			templateVariables,
			"builtin.machineDeployment.class",
		)

		// If the builtin variable could be read.
		if err != nil || !found {
			return false
		}

		// If templateMDClass matches one of the configured MachineDeploymentClasses.
		for _, mdClass := range names {
			// We have to quote mdClass as templateMDClassJSON is a JSON string (e.g. "default-worker").
			if mdClass == "*" || string(templateMDClassJSON.Raw) == strconv.Quote(mdClass) {
				return true
			}
			unquoted, _ := strconv.Unquote(string(templateMDClassJSON.Raw))
			if strings.HasPrefix(mdClass, "*") &&
				strings.HasSuffix(unquoted, strings.TrimPrefix(mdClass, "*")) {
				return true
			}
			if strings.HasSuffix(mdClass, "*") &&
				strings.HasPrefix(unquoted, strings.TrimSuffix(mdClass, "*")) {
				return true
			}
		}
	}

	return false
}
