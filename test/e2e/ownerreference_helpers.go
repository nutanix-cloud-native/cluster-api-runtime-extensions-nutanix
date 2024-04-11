//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
)

const (
	clusterKind                   = "Cluster"
	clusterClassKind              = "ClusterClass"
	machineDeploymentKind         = "MachineDeployment"
	machineSetKind                = "MachineSet"
	machineKind                   = "Machine"
	clusterResourceSetKind        = "ClusterResourceSet"
	clusterResourceSetBindingKind = "ClusterResourceSetBinding"

	awsMachineKind                   = "AWSMachine"
	awsMachineTemplateKind           = "AWSMachineTemplate"
	awsClusterKind                   = "AWSCluster"
	awsClusterTemplateKind           = "AWSClusterTemplate"
	awsClusterControllerIdentityKind = "AWSClusterControllerIdentity"

	helmChartProxyKind   = "HelmChartProxy"
	helmReleaseProxyKind = "HelmReleaseProxy"
)

var (
	coreGroupVersion = clusterv1.GroupVersion.String()

	clusterOwner = metav1.OwnerReference{
		Kind:       clusterKind,
		APIVersion: coreGroupVersion,
	}
	clusterController = metav1.OwnerReference{
		Kind:       clusterKind,
		APIVersion: coreGroupVersion,
		Controller: ptr.To(true),
	}
	clusterClassOwner = metav1.OwnerReference{
		Kind:       clusterClassKind,
		APIVersion: coreGroupVersion,
	}
	machineController = metav1.OwnerReference{
		Kind:       machineKind,
		APIVersion: coreGroupVersion,
		Controller: ptr.To(true),
	}
	clusterResourceSetOwner = metav1.OwnerReference{
		Kind:       clusterResourceSetKind,
		APIVersion: addonsv1.GroupVersion.String(),
	}
	helmChartProxyOwner = metav1.OwnerReference{
		Kind:       helmChartProxyKind,
		APIVersion: caaphv1.GroupVersion.String(),
		Controller: ptr.To(true),
	}

	// AddonReferenceAssertions maps addontypes to functions which return an error if the passed OwnerReferences
	// aren't as expected.
	AddonReferenceAssertions = map[string]func([]metav1.OwnerReference) error{
		clusterResourceSetKind: func(owners []metav1.OwnerReference) error {
			// The ClusterResourcesSets that we create are cluster specific and so should be owned by the cluster.
			return framework.HasExactOwners(
				owners,
				clusterOwner,
			)
		},

		clusterResourceSetBindingKind: clusterResourceSetBindingIsOnlyOwnedByClusterResourceSets,

		helmChartProxyKind: func(owners []metav1.OwnerReference) error {
			// The HelmChartProxies that we create are cluster specific and so should be owned by the cluster.
			return framework.HasExactOwners(
				owners,
				clusterOwner,
			)
		},

		helmReleaseProxyKind: func(owners []metav1.OwnerReference) error {
			// HelmReleaseProxies should be owned by the relevant HelmChartProxy.
			return framework.HasExactOwners(
				owners,
				helmChartProxyOwner,
			)
		},
	}

	// AWSInfraOwnerReferenceAssertions maps AWS Infrastructure types to functions which return an error if the passed
	// OwnerReferences aren't as expected.
	AWSInfraOwnerReferenceAssertions = map[string]func([]metav1.OwnerReference) error{
		awsMachineKind: func(owners []metav1.OwnerReference) error {
			// The AWSMachine must be owned and controlled by a Machine.
			return framework.HasExactOwners(owners, machineController)
		},
		awsMachineTemplateKind: func(owners []metav1.OwnerReference) error {
			// Base AWSrMachineTemplates referenced in a ClusterClass must be owned by the ClusterClass.
			// AWSMachineTemplates created for specific Clusters in the Topology controller must be owned by a Cluster.
			return framework.HasOneOfExactOwners(
				owners,
				[]metav1.OwnerReference{clusterOwner},
				[]metav1.OwnerReference{clusterClassOwner},
			)
		},
		awsClusterKind: func(owners []metav1.OwnerReference) error {
			// AWSCluster must be owned and controlled by a Cluster.
			return framework.HasExactOwners(owners, clusterController)
		},
		awsClusterTemplateKind: func(owners []metav1.OwnerReference) error {
			// AWSClusterTemplate must be owned by a ClusterClass.
			return framework.HasExactOwners(owners, clusterClassOwner)
		},
		awsClusterControllerIdentityKind: func(owners []metav1.OwnerReference) error {
			// AWSClusterControllerIdentity should have no owners.
			return framework.HasExactOwners(owners)
		},
	}
)

// dedupeOwners returns a a list of owners without duplicate owner types. Only the fields used in the
// CAPI e2e owner comparison funcs are used in the comparison here to assert equality.
func dedupeOwners(owners []metav1.OwnerReference) []metav1.OwnerReference {
	return slices.CompactFunc(owners, func(a, b metav1.OwnerReference) bool {
		return a.APIVersion == b.APIVersion && a.Kind == b.Kind &&
			ptr.Equal(a.Controller, b.Controller)
	})
}

// clusterResourceSetBindingIsOnlyOwnedByClusterResourceSets returns a function that checks that the passed
// OwnerReferences are as expected, which means only owned by ClusterResourceSets and not by any other kinds.
func clusterResourceSetBindingIsOnlyOwnedByClusterResourceSets(
	gotOwners []metav1.OwnerReference,
) error {
	return framework.HasExactOwners(dedupeOwners(gotOwners), clusterResourceSetOwner)
}
