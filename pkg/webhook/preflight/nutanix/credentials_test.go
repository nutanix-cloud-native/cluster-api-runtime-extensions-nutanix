// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	prismv3 "github.com/nutanix-cloud-native/prism-go-client/v3"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestNewCredentialsCheck_Success(t *testing.T) {
	cd := validCheckDependencies()
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &clientWrapper{
			GetCurrentLoggedInUserFunc: func(ctx context.Context) (*prismv3.UserIntentResponse, error) {
				return &prismv3.UserIntentResponse{}, nil
			},
		}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Empty(t, result.Causes)
}

func TestNewCredentialsCheck_NoNutanixConfig(t *testing.T) {
	cd := validCheckDependencies()
	cd.nutanixClusterConfigSpec = nil
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &clientWrapper{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Empty(t, result.Causes)
}

func TestNewCredentialsCheck_MissingNutanixField(t *testing.T) {
	cd := validCheckDependencies()
	cd.nutanixClusterConfigSpec.Nutanix = nil
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &clientWrapper{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.NotEmpty(t, result.Causes)
	assert.Contains(
		t,
		result.Causes[0].Message,
		"The Nutanix configuration is missing from the Cluster resource. Review the Cluster resource.", ///nolint:lll // Message is long.
	)
}

func TestNewCredentialsCheck_InvalidURL(t *testing.T) {
	cd := validCheckDependencies()
	cd.nutanixClusterConfigSpec.Nutanix.PrismCentralEndpoint.URL = "not-a-url"
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &clientWrapper{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Contains(
		t,
		result.Causes[0].Message,
		"Failed to parse the Prism Central endpoint URL \"not-a-url\": error parsing Prism Central URL: parse \"not-a-url\": invalid URI for request. Check the URL format and retry.", ///nolint:lll // Message is long.
	)
}

func TestNewCredentialsCheck_SecretNotFound(t *testing.T) {
	cd := validCheckDependencies()
	cd.kclient = fake.NewClientBuilder().Build() // no secret
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &clientWrapper{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Contains(t, result.Causes[0].Message, "Prism Central credentials Secret \"ntnx-creds\" not found")
}

type fakeK8sSecretClient struct {
	ctrlclient.Client
	getSecretFunc func(context.Context, types.NamespacedName, ctrlclient.Object, ...ctrlclient.GetOption) error
}

func (m *fakeK8sSecretClient) Get(
	ctx context.Context,
	key types.NamespacedName,
	obj ctrlclient.Object,
	opts ...ctrlclient.GetOption,
) error {
	if m.getSecretFunc != nil {
		return m.getSecretFunc(ctx, key, obj, opts...)
	}
	return nil
}

func TestNewCredentialsCheck_SecretGetError(t *testing.T) {
	cd := validCheckDependencies()
	cd.kclient = &fakeK8sSecretClient{
		getSecretFunc: func(ctx context.Context,
			key types.NamespacedName,
			obj ctrlclient.Object,
			opts ...ctrlclient.GetOption,
		) error {
			return fmt.Errorf("fake error")
		},
	}
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &clientWrapper{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.InternalError)
	assert.Contains(
		t,
		result.Causes[0].Message,
		"Failed to get Prism Central credentials Secret \"ntnx-creds\": fake error",
	)
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
		return &clientWrapper{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Contains(t, result.Causes[0].Message, "Credentials Secret \"ntnx-creds\" is empty")
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
	assert.False(t, result.InternalError)
	assert.Contains(t, result.Causes[0].Message, "does not contain key \"credentials\"")
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
		return &clientWrapper{}, nil
	}
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Contains(t, result.Causes[0].Message, "Failed to parse Prism Central credentials")
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
	assert.True(t, result.InternalError)
	assert.Contains(
		t,
		result.Causes[0].Message,
		"Failed to initialize the Nutanix Prism Central API client: assert.AnError general error for testing.", ///nolint:lll // Message is long.
	)
}

func TestNewCredentialsCheck_FailedToGetCurrentLoggedInUser(t *testing.T) {
	// Simulate a failure in getting the current logged-in user
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &clientWrapper{
			GetCurrentLoggedInUserFunc: func(ctx context.Context) (*prismv3.UserIntentResponse, error) {
				return nil, assert.AnError
			},
		}, nil
	}
	cd := validCheckDependencies()
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.InternalError)
	assert.Contains(t, result.Causes[0].Message, "Failed to validate credentials: "+
		assert.AnError.Error())
}

func TestNewCredentialsCheck_GetCurrentLoggedInUserInvalidCredentials(t *testing.T) {
	nclientFactory := func(_ prismgoclient.Credentials) (client, error) {
		return &clientWrapper{
			GetCurrentLoggedInUserFunc: func(ctx context.Context) (*prismv3.UserIntentResponse, error) {
				return nil, fmt.Errorf("invalid Nutanix credentials")
			},
		}, nil
	}
	cd := validCheckDependencies()
	check := newCredentialsCheck(context.Background(), nclientFactory, cd)
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Contains(t, result.Causes[0].Message, "Failed to validate credentials: "+
		"invalid Nutanix credentials")
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
