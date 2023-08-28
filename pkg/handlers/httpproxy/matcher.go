// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// This code re-implements private matcher for CAPI inline patch selector.
// See: https://github.com/kubernetes-sigs/cluster-api/blob/46412f0a4ea65d8f02478d2ad09ce12925485f56/api/v1beta1/clusterclass_types.go#L509-L523
// See: https://github.com/kubernetes-sigs/cluster-api/blob/46412f0a4ea65d8f02478d2ad09ce12925485f56/internal/controllers/topology/cluster/patches/inline/json_patch_generator.go#L125
package httpproxy

import (
	"strconv"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
)

func matchSelector(
	selector clusterv1.PatchSelector,
	obj runtime.Object,
	holderRef runtimehooksv1.HolderReference,
	templateVariables map[string]apiextensionsv1.JSON,
) bool {
	if !matchGVK(selector.APIVersion, selector.Kind, obj) {
		return false
	}

	if selector.MatchResources.InfrastructureCluster {
		if !matchInfrastructure(holderRef) {
			return false
		}
	}

	if selector.MatchResources.ControlPlane {
		if !matchControlPlane(holderRef) {
			return false
		}
	}

	if selector.MatchResources.MachineDeploymentClass != nil {
		if !matchMachineDeploymentClass(
			holderRef,
			selector.MatchResources.MachineDeploymentClass.Names,
			templateVariables,
		) {
			return false
		}
	}

	return true
}

// Check if the apiVersion and kind are matching.
func matchGVK(apiVersion, kind string, obj runtime.Object) bool {
	objApiVersion, objKind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	// Check if the apiVersion and kind are matching.
	if objApiVersion != apiVersion {
		return false
	}
	if objKind != kind {
		return false
	}
	return true
}

// Check if the request is for an InfrastructureCluster.
func matchInfrastructure(holderRef runtimehooksv1.HolderReference) bool {
	// Cluster.spec.infrastructureRef holds the InfrastructureCluster.
	if holderRef.Kind == "Cluster" && holderRef.FieldPath == "spec.infrastructureRef" {
		return true
	}
	return false
}

// Check if the request is for a ControlPlane or the InfrastructureMachineTemplate of a ControlPlane.
func matchControlPlane(holderRef runtimehooksv1.HolderReference) bool {
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

// Check if the request is for a BootstrapConfigTemplate or an InfrastructureMachineTemplate
// of one of the configured MachineDeploymentClasses.
func matchMachineDeploymentClass(
	holderRef runtimehooksv1.HolderReference,
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
