// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package generic

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/types/ping"
	"github.com/regclient/regclient/types/ref"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

var (
	//nolint:errname,staticcheck  // this is an internal test error not for consumption
	testPingFailedError = fmt.Errorf("ping failed")
	testRegistryURL     = "https://artifactory.canaveral-corp.us-west-2.aws"
)

type mockRegClient struct {
	pingFunc func(ref.Ref) error
}

//nolint:gocritic  // this is how the method is defined in regClient
func (m *mockRegClient) Ping(ctx context.Context, r ref.Ref) (ping.Result, error) {
	err := fmt.Errorf("failed to set pingFunc")
	if m.pingFunc != nil {
		err = m.pingFunc(r)
	}
	return ping.Result{}, err
}

type mockK8sClient struct {
	ctrlclient.Client
	getSecretFunc func(context.Context, types.NamespacedName, ctrlclient.Object, ...ctrlclient.GetOption) error
}

func (m *mockK8sClient) Get(
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

func TestRegistryCheck(t *testing.T) {
	testCases := []struct {
		name                       string
		registryMirror             *carenv1.GlobalImageRegistryMirror
		imageRegistry              *carenv1.ImageRegistry
		kclient                    ctrlclient.Client
		mockRegClientPingerFactory regClientPingerFactory
		want                       preflight.CheckResult
	}{
		{
			name: "no registry configuration",
			want: preflight.CheckResult{
				Allowed: true,
			},
		},
		{
			name: "registry mirror with valid credentials",
			registryMirror: &carenv1.GlobalImageRegistryMirror{
				URL: testRegistryURL,
				Credentials: &carenv1.RegistryCredentials{
					SecretRef: &carenv1.LocalObjectReference{
						Name: "test-secret",
					},
				},
			},
			kclient: &mockK8sClient{
				getSecretFunc: func(ctx context.Context,
					key types.NamespacedName,
					obj ctrlclient.Object,
					opts ...ctrlclient.GetOption,
				) error {
					secret := obj.(*corev1.Secret)
					secret.Data = map[string][]byte{
						"username": []byte("testuser"),
						"password": []byte("testpass"),
					}
					return nil
				},
			},
			mockRegClientPingerFactory: func(...regclient.Opt) regClientPinger {
				return &mockRegClient{
					pingFunc: func(ref.Ref) error { return nil },
				}
			},
			want: preflight.CheckResult{
				Allowed: true,
			},
		},
		{
			name: "registry mirror with invalid credentials secret",
			registryMirror: &carenv1.GlobalImageRegistryMirror{
				URL: testRegistryURL,
				Credentials: &carenv1.RegistryCredentials{
					SecretRef: &carenv1.LocalObjectReference{
						Name: "test-secret",
					},
				},
			},
			kclient: &mockK8sClient{
				getSecretFunc: func(ctx context.Context,
					key types.NamespacedName,
					obj ctrlclient.Object,
					opts ...ctrlclient.GetOption,
				) error {
					return fmt.Errorf("secret not found")
				},
			},
			want: preflight.CheckResult{
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "failed to get Registry credentials Secret: secret not found",
						//nolint:lll // this is a test for a field.
						Field: "cluster.spec.topology.variables[.name=clusterConfig].value.globalImageRegistryMirror.credentials.secretRef",
					},
				},
			},
		},
		{
			name: "registry mirror with missing username in secret",
			registryMirror: &carenv1.GlobalImageRegistryMirror{
				URL: testRegistryURL,
				Credentials: &carenv1.RegistryCredentials{
					SecretRef: &carenv1.LocalObjectReference{
						Name: "test-secret",
					},
				},
			},
			kclient: &mockK8sClient{
				getSecretFunc: func(ctx context.Context,
					key types.NamespacedName,
					obj ctrlclient.Object,
					opts ...ctrlclient.GetOption,
				) error {
					secret := obj.(*corev1.Secret)
					secret.Data = map[string][]byte{
						"password": []byte("testpass"),
					}
					return nil
				},
			},
			want: preflight.CheckResult{
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "failed to get username from Registry credentials Secret. secret must have field username.",
						//nolint:lll // this is a test for a field.
						Field: "cluster.spec.topology.variables[.name=clusterConfig].value.globalImageRegistryMirror.credentials.secretRef",
					},
				},
			},
		},
		{
			name: "registry mirror with missing password in secret",
			registryMirror: &carenv1.GlobalImageRegistryMirror{
				URL: testRegistryURL,
				Credentials: &carenv1.RegistryCredentials{
					SecretRef: &carenv1.LocalObjectReference{
						Name: "test-secret",
					},
				},
			},
			kclient: &mockK8sClient{
				getSecretFunc: func(ctx context.Context,
					key types.NamespacedName,
					obj ctrlclient.Object,
					opts ...ctrlclient.GetOption,
				) error {
					secret := obj.(*corev1.Secret)
					secret.Data = map[string][]byte{
						"username": []byte("testuser"),
					}
					return nil
				},
			},
			want: preflight.CheckResult{
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "failed to get password from Registry credentials Secret. secret must have field password.",
						//nolint:lll // this is a test for a field.
						Field: "cluster.spec.topology.variables[.name=clusterConfig].value.globalImageRegistryMirror.credentials.secretRef",
					},
				},
			},
		},
		{
			name: "registry mirror ping failure",
			registryMirror: &carenv1.GlobalImageRegistryMirror{
				URL: testRegistryURL,
			},
			mockRegClientPingerFactory: func(...regclient.Opt) regClientPinger {
				return &mockRegClient{
					pingFunc: func(ref.Ref) error { return testPingFailedError },
				}
			},
			want: preflight.CheckResult{
				Allowed: false,
				Causes: []preflight.Cause{
					{
						Message: pingFailedReasonString(
							mirrorURLValidationRegex.ReplaceAllString(testRegistryURL, ""),
							testPingFailedError,
						),
						Field: "cluster.spec.topology.variables[.name=clusterConfig].value.globalImageRegistryMirror",
					},
				},
			},
		},
		{
			name: "image registry with valid configuration",
			imageRegistry: &carenv1.ImageRegistry{
				URL: "https://registry.example.com",
				Credentials: &carenv1.RegistryCredentials{
					SecretRef: &carenv1.LocalObjectReference{
						Name: "test-secret",
					},
				},
			},
			kclient: &mockK8sClient{
				getSecretFunc: func(ctx context.Context,
					key types.NamespacedName,
					obj ctrlclient.Object,
					opts ...ctrlclient.GetOption,
				) error {
					secret := obj.(*corev1.Secret)
					secret.Data = map[string][]byte{
						"username": []byte("testuser"),
						"password": []byte("testpass"),
						"ca.crt":   []byte("test-ca-cert"),
					}
					return nil
				},
			},
			mockRegClientPingerFactory: func(...regclient.Opt) regClientPinger {
				return &mockRegClient{
					pingFunc: func(ref.Ref) error { return nil },
				}
			},
			want: preflight.CheckResult{
				Allowed: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test cluster
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
				},
			}

			// Create the check
			check := &registryCheck{
				registryMirror:        tc.registryMirror,
				imageRegistry:         tc.imageRegistry,
				kclient:               tc.kclient,
				cluster:               cluster,
				regClientPingerGetter: tc.mockRegClientPingerFactory,
			}

			// Execute the check
			got := check.Run(context.Background())

			// Verify the result
			assert.Equal(t, tc.want.Allowed, got.Allowed, "(allowed) mismatch for test "+tc.name)
			assert.Equal(t, tc.want.Error, got.Error, "(error) mismatch test "+tc.name)
			assert.Equal(t, tc.want.Causes, got.Causes, "(causes) mismatch test "+tc.name)
		})
	}
}

func TestNewRegistryCheck(t *testing.T) {
	testCases := []struct {
		name                       string
		genericClusterConfigSpec   *carenv1.GenericClusterConfigSpec
		expectedChecks             int
		expectRegistryMirrorCheck  bool
		expectImageRegistriesCheck bool
	}{
		{
			name:                       "no registry configuration",
			genericClusterConfigSpec:   &carenv1.GenericClusterConfigSpec{},
			expectedChecks:             0,
			expectRegistryMirrorCheck:  false,
			expectImageRegistriesCheck: false,
		},
		{
			name: "only registry mirror configuration",
			genericClusterConfigSpec: &carenv1.GenericClusterConfigSpec{
				GlobalImageRegistryMirror: &carenv1.GlobalImageRegistryMirror{
					URL: testRegistryURL,
				},
			},
			expectedChecks:             1,
			expectRegistryMirrorCheck:  true,
			expectImageRegistriesCheck: false,
		},
		{
			name: "only image registries configuration",
			genericClusterConfigSpec: &carenv1.GenericClusterConfigSpec{
				ImageRegistries: []carenv1.ImageRegistry{
					{
						URL: "https://registry1.example.com",
					},
					{
						URL: "https://registry2.example.com",
					},
				},
			},
			expectedChecks:             2,
			expectRegistryMirrorCheck:  false,
			expectImageRegistriesCheck: true,
		},
		{
			name: "both registry mirror and image registries configuration",
			genericClusterConfigSpec: &carenv1.GenericClusterConfigSpec{
				GlobalImageRegistryMirror: &carenv1.GlobalImageRegistryMirror{
					URL: testRegistryURL,
				},
				ImageRegistries: []carenv1.ImageRegistry{
					{
						URL: "https://registry1.example.com",
					},
				},
			},
			expectedChecks:             2,
			expectRegistryMirrorCheck:  true,
			expectImageRegistriesCheck: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cd := &checkDependencies{
				genericClusterConfigSpec: tc.genericClusterConfigSpec,
				cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-ns",
					},
				},
				kclient: &mockK8sClient{},
				log:     testr.New(t),
			}

			checks := newRegistryCheck(cd)

			assert.Len(t, checks, tc.expectedChecks)

			for _, check := range checks {
				rc, ok := check.(*registryCheck)
				require.True(t, ok)
				if tc.expectRegistryMirrorCheck && rc.imageRegistry == nil { //
					assert.NotNil(
						t,
						rc.registryMirror,
						"expected registry mirror got %v for field %s", rc.registryMirror, rc.field,
					)
				} else if tc.expectImageRegistriesCheck {
					assert.NotNil(t, rc.imageRegistry, "expected image registry got %v for field %s", rc.imageRegistry, rc.field)
				}
			}
		})
	}
}
