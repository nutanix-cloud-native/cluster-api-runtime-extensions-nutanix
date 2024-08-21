//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	clusterctlcluster "sigs.k8s.io/cluster-api/cmd/clusterctl/client/cluster"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
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
	kubeadmControlPlaneKind       = "KubeadmControlPlane"
	kubeadmConfigKind             = "KubeadmConfig"

	awsMachineKind                   = "AWSMachine"
	awsMachineTemplateKind           = "AWSMachineTemplate"
	awsClusterKind                   = "AWSCluster"
	awsClusterTemplateKind           = "AWSClusterTemplate"
	awsClusterControllerIdentityKind = "AWSClusterControllerIdentity"

	nutanixMachineKind         = "NutanixMachine"
	nutanixMachineTemplateKind = "NutanixMachineTemplate"
	nutanixClusterKind         = "NutanixCluster"
	nutanixClusterTemplateKind = "NutanixClusterTemplate"

	helmChartProxyKind   = "HelmChartProxy"
	helmReleaseProxyKind = "HelmReleaseProxy"

	secretKind    = "Secret"
	configMapKind = "ConfigMap"
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
	kubeadmControlPlaneGroupVersion = controlplanev1.GroupVersion.String()
	kubeadmControlPlaneController   = metav1.OwnerReference{
		Kind:       kubeadmControlPlaneKind,
		APIVersion: kubeadmControlPlaneGroupVersion,
		Controller: ptr.To(true),
	}
	kubeadmConfigGroupVersion = bootstrapv1.GroupVersion.String()
	kubeadmConfigController   = metav1.OwnerReference{
		Kind:       kubeadmConfigKind,
		APIVersion: kubeadmConfigGroupVersion,
		Controller: ptr.To(true),
	}

	capxGroupVersion    = capxv1.GroupVersion.String()
	nutanixClusterOwner = metav1.OwnerReference{
		Kind:       nutanixClusterKind,
		APIVersion: capxGroupVersion,
	}

	// AddonReferenceAssertions maps addon types to functions which return an error if the passed OwnerReferences
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
			// Base AWSMachineTemplates referenced in a ClusterClass must be owned by the ClusterClass.
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

	// NutanixInfraOwnerReferenceAssertions maps Nutanix Infrastructure types to functions which return an error
	// if the passed OwnerReferences aren't as expected.
	NutanixInfraOwnerReferenceAssertions = map[string]func([]metav1.OwnerReference) error{
		nutanixMachineKind: func(owners []metav1.OwnerReference) error {
			// The NutanixMachine must be owned and controlled by a Machine.
			return framework.HasExactOwners(owners, machineController)
		},
		nutanixMachineTemplateKind: func(owners []metav1.OwnerReference) error {
			// Base NutanixMachineTemplates referenced in a ClusterClass must be owned by the ClusterClass.
			// NutanixMachineTemplates created for specific Clusters in the Topology controller must be owned by a Cluster.
			return framework.HasOneOfExactOwners(
				owners,
				[]metav1.OwnerReference{clusterOwner},
				[]metav1.OwnerReference{clusterClassOwner},
			)
		},
		nutanixClusterKind: func(owners []metav1.OwnerReference) error {
			// NutanixCluster must be owned and controlled by a Cluster.
			return framework.HasExactOwners(owners, clusterController)
		},
		nutanixClusterTemplateKind: func(owners []metav1.OwnerReference) error {
			// NutanixClusterTemplate must be owned by a ClusterClass.
			return framework.HasExactOwners(owners, clusterClassOwner)
		},
	}

	// KubernetesReferenceAssertions maps Kubernetes types to functions which return an error if the passed OwnerReferences
	// aren't as expected.
	// Note: These relationships are documented in
	// https://github.com/kubernetes-sigs/cluster-api/tree/main/docs/book/src/reference/owner_references.md.
	KubernetesReferenceAssertions = map[string]func([]metav1.OwnerReference) error{
		secretKind: func(owners []metav1.OwnerReference) error {
			// Secrets for cluster certificates must be owned and controlled by the KubeadmControlPlane.
			// The bootstrap secret should be owned and controlled by a KubeadmControlPlane.
			// Other resources can be owned by the Cluster to ensure correct GC.
			return framework.HasOneOfExactOwners(
				owners,
				[]metav1.OwnerReference{kubeadmControlPlaneController},
				[]metav1.OwnerReference{kubeadmConfigController},
				[]metav1.OwnerReference{clusterOwner},
				[]metav1.OwnerReference{clusterOwner, nutanixClusterOwner},
			)
		},
		configMapKind: func(owners []metav1.OwnerReference) error {
			// The only configMaps considered here are those owned by a ClusterResourceSet.
			return framework.HasExactOwners(owners, clusterResourceSetOwner)
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

// filterClusterObjectsWithNameFilterIgnoreClusterAutoscaler filters out all objects that are not part of the workload
// cluster hierarchy as well as cluster autoscaler objects because they are not owned by any cluster object.
func filterClusterObjectsWithNameFilterIgnoreClusterAutoscaler(
	clusterName string,
) clusterctlcluster.GetOwnerGraphFilterFunction {
	return func(u unstructured.Unstructured) bool {
		// If the resource does not contain the cluster name in its name, it is not part of the workload cluster hierarchy
		// so filter it out.
		if clusterctlcluster.FilterClusterObjectsWithNameFilter(clusterName)(u) {
			return false
		}

		// Filter out the cluster-autoscaler resources because they are not owned by any cluster object.
		// These check is dependent on the implementation of the cluster-autoscaler addon
		// and as such is fragile.
		if u.GetKind() == clusterResourceSetKind ||
			u.GetKind() == helmChartProxyKind {
			if u.GetName() == "cluster-autoscaler-"+clusterName {
				return false
			}
		}

		return true
	}
}
