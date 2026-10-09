// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanixflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestFlowCNIRemoteNamespaces(t *testing.T) {
	t.Parallel()

	// Flow CNI 1.1.0 deploys Helm hooks and private images into three
	// namespaces. The handler ensures each exists before HelmChartProxy apply.
	assert.ElementsMatch(t, []string{
		"flow-cni-system",
		"flow-cns-system",
		"ovn-kubernetes",
	}, flowCNIRemoteNamespaces)
}

func TestEnsureFlowCNIRemoteNamespaces_withoutImagePullCredentials(t *testing.T) {
	t.Parallel()

	// ImagePullCredentials == nil must still create hook namespaces; previously
	// they were only created as a side effect of copying the pull secret.
	cniVar := v1alpha1.CNI{
		Provider: v1alpha1.CNIProviderFlow,
		Strategy: v1alpha1.AddonStrategyHelmAddon,
	}
	require.Nil(t, cniVar.ImagePullCredentials)

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	remoteClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := ensureFlowCNIRemoteNamespaces(context.Background(), remoteClient)
	require.NoError(t, err)

	for _, ns := range flowCNIRemoteNamespaces {
		got := &corev1.Namespace{}
		err := remoteClient.Get(context.Background(), ctrlclient.ObjectKey{Name: ns}, got)
		require.NoError(t, err, "expected namespace %q to exist", ns)
		assert.Equal(t, ns, got.Name)
	}
}
