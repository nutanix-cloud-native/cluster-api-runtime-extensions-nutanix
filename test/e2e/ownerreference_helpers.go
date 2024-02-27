//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	caaphv1 "github.com/d2iq-labs/capi-runtime-extensions/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
)

const (
	clusterKind            = "Cluster"
	clusterResourceSetKind = "ClusterResourceSet"
	helmChartProxyKind     = "HelmChartProxy"
	helmReleaseProxyKind   = "HelmReleaseProxy"
)

var (
	clusterOwner = metav1.OwnerReference{
		Kind:       clusterKind,
		APIVersion: clusterv1.GroupVersion.String(),
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
)
