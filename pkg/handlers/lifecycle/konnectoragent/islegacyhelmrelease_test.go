// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package konnectoragent

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func TestIsLegacyHelmRelease_WithHelmReleaseSecretYAML(t *testing.T) {
	t.Parallel()

	// Read the helm release secret YAML file
	// The file is in the same directory as this test file
	_, testFile, _, _ := goruntime.Caller(0)
	testDir := filepath.Dir(testFile)
	helmSecretPath := filepath.Join(testDir, "helmreleasesecret.yaml")

	yamlData, err := os.ReadFile(helmSecretPath)
	require.NoError(t, err, "Failed to read helm release secret YAML file from: %s", helmSecretPath)
	require.NotEmpty(t, yamlData, "Helm release secret YAML file is empty")
	t.Logf("Successfully read helm release secret from: %s", helmSecretPath)

	// Parse the YAML into a Secret object
	secret := &corev1.Secret{}
	err = yaml.Unmarshal(yamlData, secret)
	require.NoError(t, err, "Failed to unmarshal helm release secret YAML")
	require.NotNil(t, secret, "Secret is nil after unmarshaling")
	require.NotEmpty(t, secret.Data["release"], "Secret does not contain 'release' data")

	// Create a test handler
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	handler := &DefaultKonnectorAgent{
		client: client,
	}

	// Test that the function correctly identifies the legacy helm release
	// The chart name should be "nutanix-k8s-agent" (not the release name "k8sagent")
	result := handler.isLegacyHelmRelease(secret)

	assert.True(
		t,
		result,
		"isLegacyHelmRelease should return true for helm release with chart name 'nutanix-k8s-agent'",
	)
	t.Logf("Successfully identified legacy helm release. Secret name: %s, Namespace: %s", secret.Name, secret.Namespace)
}

func TestIsLegacyHelmRelease_WithNonLegacyRelease(t *testing.T) {
	t.Parallel()

	// Create a test handler
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	handler := &DefaultKonnectorAgent{
		client: client,
	}

	// Create a secret that is NOT a legacy helm release
	// This secret has a different chart name
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1.other-chart.v1",
			Namespace: "ntnx-system",
		},
		Data: map[string][]byte{
			"release": []byte("dGVzdCBkYXRhIHdpdGggb3RoZXIgY2hhcnQgbmFtZQ=="), // base64 encoded test data
		},
		Type: "helm.sh/release.v1",
	}

	result := handler.isLegacyHelmRelease(secret)
	assert.False(t, result, "isLegacyHelmRelease should return false for non-legacy helm release")
}

func TestIsLegacyHelmRelease_WithMissingReleaseData(t *testing.T) {
	t.Parallel()

	// Create a test handler
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	handler := &DefaultKonnectorAgent{
		client: client,
	}

	// Create a secret without release data
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1.test.v1",
			Namespace: "ntnx-system",
		},
		Data: map[string][]byte{},
		Type: "helm.sh/release.v1",
	}

	result := handler.isLegacyHelmRelease(secret)
	assert.False(t, result, "isLegacyHelmRelease should return false when release data is missing")
}
