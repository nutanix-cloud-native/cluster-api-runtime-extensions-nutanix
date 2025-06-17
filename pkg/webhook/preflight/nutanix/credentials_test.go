// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestNewCredentialsCheck_Success(t *testing.T) {
	cd := validCheckDependencies()
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &mocknclient{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.Error)
	assert.Empty(t, result.Causes)
}

func TestNewCredentialsCheck_NoNutanixConfig(t *testing.T) {
	cd := validCheckDependencies()
	cd.nutanixClusterConfigSpec = nil
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &mocknclient{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.Error)
	assert.Empty(t, result.Causes)
}

func TestNewCredentialsCheck_MissingNutanixField(t *testing.T) {
	cd := validCheckDependencies()
	cd.nutanixClusterConfigSpec.Nutanix = nil
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &mocknclient{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.NotEmpty(t, result.Causes)
	assert.Contains(t, result.Causes[0].Message, "Nutanix cluster configuration is not defined")
}

func TestNewCredentialsCheck_InvalidURL(t *testing.T) {
	cd := validCheckDependencies()
	cd.nutanixClusterConfigSpec.Nutanix.PrismCentralEndpoint.URL = "not-a-url"
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &mocknclient{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "failed to parse Prism Central endpoint URL")
}

func TestNewCredentialsCheck_SecretNotFound(t *testing.T) {
	cd := validCheckDependencies()
	cd.kclient = fake.NewClientBuilder().Build() // no secret
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &mocknclient{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "failed to get Prism Central credentials Secret")
}

func TestNewCredentialsCheck_SecretEmpty(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ntnx-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{},
	}
	cd := validCheckDependencies()
	cd.kclient = fake.NewClientBuilder().WithObjects(secret).Build()
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &mocknclient{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "credentials Secret 'ntnx-creds' is empty")
}

func TestNewCredentialsCheck_SecretMissingKey(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ntnx-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"not-credentials": []byte("foo"),
		},
	}
	cd := validCheckDependencies()
	cd.kclient = fake.NewClientBuilder().WithObjects(secret).Build()
	check := newCredentialsCheck(context.Background(), nil, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "does not contain key 'credentials'")
}

func TestNewCredentialsCheck_InvalidCredentialsFormat(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ntnx-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("not-a-valid-format"),
		},
	}
	cd := validCheckDependencies()
	cd.kclient = fake.NewClientBuilder().WithObjects(secret).Build()
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &mocknclient{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "failed to parse Prism Central credentials")
}

func TestNewCredentialsCheck_FailedToCreateClient(t *testing.T) {
	// Simulate a failure in creating the v4 client
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return nil, assert.AnError
	}
	cd := validCheckDependencies()
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "Failed to initialize Nutanix client")
}

func TestNewCredentialsCheck_FailedToGetCurrentLoggedInUser(t *testing.T) {
	// Simulate a failure in getting the current logged-in user
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &mocknclient{err: assert.AnError}, nil
	}
	cd := validCheckDependencies()
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "Failed to validate credentials using the v3 API client.")
}

func validCheckDependencies() *checkDependencies {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ntnx-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte(`[
				{
					"type": "basic_auth",
					"data": {
						"prismCentral": {
							"username": "testuser",
							"password": "testpassword"
						}
					}
				}
			]`),
		},
	}

	return &checkDependencies{
		kclient: fake.NewClientBuilder().WithObjects(secret).Build(),
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
		},
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			Nutanix: &carenv1.NutanixSpec{
				PrismCentralEndpoint: carenv1.NutanixPrismCentralEndpointSpec{
					URL: "https://pc.example.com:9440",
					Credentials: carenv1.NutanixPrismCentralEndpointCredentials{
						SecretRef: carenv1.LocalObjectReference{
							Name: "ntnx-creds",
						},
					},
				},
			},
		},
		nutanixWorkerNodeConfigSpecByMachineDeploymentName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
	}
}
