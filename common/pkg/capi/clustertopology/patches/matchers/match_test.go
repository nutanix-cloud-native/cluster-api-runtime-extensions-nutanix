// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package matchers_test

import (
	"testing"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/matchers"
)

func TestMatchesSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		obj               runtime.Object
		templateVariables map[string]apiextensionsv1.JSON
		holderRef         *runtimehooksv1.HolderReference
		selector          clusterv1.PatchSelector
		match             bool
	}{{
		name: "Don't match: apiVersion mismatch",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
				"kind":       "AzureMachineTemplate",
			},
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
			Kind:       "AzureMachineTemplate",
		},
		match: false,
	}, {
		name: "Don't match: kind mismatch",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
				"kind":       "AzureMachineTemplate",
			},
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			Kind:       "AzureClusterTemplate",
		},
		match: false,
	}, {
		name: "Match InfrastructureClusterTemplate",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
				"kind":       "AzureClusterTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       "my-cluster",
			Namespace:  "default",
			FieldPath:  "spec.infrastructureRef",
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			Kind:       "AzureClusterTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				InfrastructureCluster: true,
			},
		},
		match: true,
	}, {
		name: "Don't match InfrastructureClusterTemplate, .matchResources.infrastructureCluster not set",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
				"kind":       "AzureClusterTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       "my-cluster",
			Namespace:  "default",
			FieldPath:  "spec.infrastructureRef",
		},
		selector: clusterv1.PatchSelector{
			APIVersion:     "infrastructure.cluster.x-k8s.io/v1beta1",
			Kind:           "AzureClusterTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{},
		},
		match: false,
	}, {
		name: "Don't match InfrastructureClusterTemplate, .matchResources.infrastructureCluster false",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
				"kind":       "AzureClusterTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       "my-cluster",
			Namespace:  "default",
			FieldPath:  "spec.infrastructureRef",
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			Kind:       "AzureClusterTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				InfrastructureCluster: false,
			},
		},
		match: false,
	}, {
		name: "Match ControlPlaneTemplate",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "controlplane.cluster.x-k8s.io/v1beta1",
				"kind":       "ControlPlaneTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       "my-cluster",
			Namespace:  "default",
			FieldPath:  "spec.controlPlaneRef",
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
			Kind:       "ControlPlaneTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				ControlPlane: true,
			},
		},
		match: true,
	}, {
		name: "Don't match ControlPlaneTemplate, .matchResources.controlPlane not set",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "controlplane.cluster.x-k8s.io/v1beta1",
				"kind":       "ControlPlaneTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       "my-cluster",
			Namespace:  "default",
			FieldPath:  "spec.controlPlaneRef",
		},
		selector: clusterv1.PatchSelector{
			APIVersion:     "controlplane.cluster.x-k8s.io/v1beta1",
			Kind:           "ControlPlaneTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{},
		},
		match: false,
	}, {
		name: "Don't match ControlPlaneTemplate, .matchResources.controlPlane false",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "controlplane.cluster.x-k8s.io/v1beta1",
				"kind":       "ControlPlaneTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       "my-cluster",
			Namespace:  "default",
			FieldPath:  "spec.controlPlaneRef",
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
			Kind:       "ControlPlaneTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				ControlPlane: false,
			},
		},
		match: false,
	}, {
		name: "Match ControlPlane InfrastructureMachineTemplate",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
				"kind":       "AzureMachineTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
			Kind:       "KubeadmControlPlane",
			Name:       "my-controlplane",
			Namespace:  "default",
			FieldPath:  "spec.machineTemplate.infrastructureRef",
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			Kind:       "AzureMachineTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				ControlPlane: true,
			},
		},
		match: true,
	}, {
		name: "Match MD BootstrapTemplate",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
				"kind":       "BootstrapTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       "my-md-0",
			Namespace:  "default",
			FieldPath:  "spec.template.spec.bootstrap.configRef",
		},
		templateVariables: map[string]apiextensionsv1.JSON{
			runtimehooksv1.BuiltinsName: {Raw: []byte(`{"machineDeployment":{"class":"classA"}}`)},
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
			Kind:       "BootstrapTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
					Names: []string{"classA"},
				},
			},
		},
		match: true,
	}, {
		name: "Match all MD BootstrapTemplate",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
				"kind":       "BootstrapTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       "my-md-0",
			Namespace:  "default",
			FieldPath:  "spec.template.spec.bootstrap.configRef",
		},
		templateVariables: map[string]apiextensionsv1.JSON{
			runtimehooksv1.BuiltinsName: {Raw: []byte(`{"machineDeployment":{"class":"classA"}}`)},
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
			Kind:       "BootstrapTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
					Names: []string{"*"},
				},
			},
		},
		match: true,
	}, {
		name: "Glob match MD BootstrapTemplate with <string>-*",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
				"kind":       "BootstrapTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       "my-md-0",
			Namespace:  "default",
			FieldPath:  "spec.template.spec.bootstrap.configRef",
		},
		templateVariables: map[string]apiextensionsv1.JSON{
			runtimehooksv1.BuiltinsName: {Raw: []byte(`{"machineDeployment":{"class":"class-A"}}`)},
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
			Kind:       "BootstrapTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
					Names: []string{"class-*"},
				},
			},
		},
		match: true,
	}, {
		name: "Glob match MD BootstrapTemplate with *-<string>",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
				"kind":       "BootstrapTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       "my-md-0",
			Namespace:  "default",
			FieldPath:  "spec.template.spec.bootstrap.configRef",
		},
		templateVariables: map[string]apiextensionsv1.JSON{
			runtimehooksv1.BuiltinsName: {Raw: []byte(`{"machineDeployment":{"class":"class-A"}}`)},
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
			Kind:       "BootstrapTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
					Names: []string{"*-A"},
				},
			},
		},
		match: true,
	}, {
		name: "Don't match BootstrapTemplate, .matchResources.machineDeploymentClass.names is empty",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
				"kind":       "BootstrapTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       "my-md-0",
			Namespace:  "default",
			FieldPath:  "spec.template.spec.bootstrap.configRef",
		},
		templateVariables: map[string]apiextensionsv1.JSON{
			runtimehooksv1.BuiltinsName: {Raw: []byte(`{"machineDeployment":{"class":"classA"}}`)},
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
			Kind:       "BootstrapTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
					Names: []string{},
				},
			},
		},
		match: false,
	}, {
		name: "Do not match BootstrapTemplate, .matchResources.machineDeploymentClass is set to nil",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
				"kind":       "BootstrapTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       "my-md-0",
			Namespace:  "default",
			FieldPath:  "spec.template.spec.bootstrap.configRef",
		},
		templateVariables: map[string]apiextensionsv1.JSON{
			runtimehooksv1.BuiltinsName: {Raw: []byte(`{"machineDeployment":{"class":"classA"}}`)},
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
			Kind:       "BootstrapTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				MachineDeploymentClass: nil,
			},
		},
		match: false,
	}, {
		name: "Don't match BootstrapTemplate, .matchResources.machineDeploymentClass not set",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
				"kind":       "BootstrapTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       "my-md-0",
			Namespace:  "default",
			FieldPath:  "spec.template.spec.bootstrap.configRef",
		},
		templateVariables: map[string]apiextensionsv1.JSON{
			runtimehooksv1.BuiltinsName: {Raw: []byte(`{"machineDeployment":{"class":"classA"}}`)},
		},
		selector: clusterv1.PatchSelector{
			APIVersion:     "bootstrap.cluster.x-k8s.io/v1beta1",
			Kind:           "BootstrapTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{},
		},
		match: false,
	}, {
		name: "Don't match BootstrapTemplate, .matchResources.machineDeploymentClass does not match",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
				"kind":       "BootstrapTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       "my-md-0",
			Namespace:  "default",
			FieldPath:  "spec.template.spec.bootstrap.configRef",
		},
		templateVariables: map[string]apiextensionsv1.JSON{
			runtimehooksv1.BuiltinsName: {Raw: []byte(`{"machineDeployment":{"class":"classA"}}`)},
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
			Kind:       "BootstrapTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
					Names: []string{"classB"},
				},
			},
		},
		match: false,
	}, {
		name: "Match MD InfrastructureMachineTemplate",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
				"kind":       "AzureMachineTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       "my-md-0",
			Namespace:  "default",
			FieldPath:  "spec.template.spec.infrastructureRef",
		},
		templateVariables: map[string]apiextensionsv1.JSON{
			runtimehooksv1.BuiltinsName: {Raw: []byte(`{"machineDeployment":{"class":"classA"}}`)},
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			Kind:       "AzureMachineTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
					Names: []string{"classA"},
				},
			},
		},
		match: true,
	}, {
		name: "Don't match: unknown field path",
		obj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "controlplane.cluster.x-k8s.io/v1beta1",
				"kind":       "ControlPlaneTemplate",
			},
		},
		holderRef: &runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Custom",
			Name:       "my-md-0",
			Namespace:  "default",
			FieldPath:  "spec.machineTemplate.unknown.infrastructureRef",
		},
		selector: clusterv1.PatchSelector{
			APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
			Kind:       "ControlPlaneTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				ControlPlane: true,
			},
		},
		match: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			g.Expect(matchers.MatchesSelector(tt.selector, tt.obj, tt.holderRef, tt.templateVariables)).
				To(Equal(tt.match))
		})
	}
}
