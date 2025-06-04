// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type mockCheck struct {
	name   string
	result preflight.CheckResult
}

func (m *mockCheck) Name() string {
	return m.name
}

func (m *mockCheck) Run(ctx context.Context) preflight.CheckResult {
	return m.result
}

func TestNutanixChecker_Init(t *testing.T) {
	tests := []struct {
		name                       string
		nutanixConfig              *carenv1.NutanixClusterConfigSpec
		workerNodeConfigs          map[string]*carenv1.NutanixWorkerNodeConfigSpec
		expectedCheckCount         int
		expectedFirstCheckName     string
		expectedSecondCheckName    string
		vmImageCheckCount          int
		storageContainerCheckCount int
	}{
		{
			name:                       "basic initialization with no configs",
			nutanixConfig:              nil,
			workerNodeConfigs:          nil,
			expectedCheckCount:         2, // config check and credentials check
			expectedFirstCheckName:     "NutanixConfiguration",
			expectedSecondCheckName:    "NutanixCredentials",
			vmImageCheckCount:          0,
			storageContainerCheckCount: 0,
		},
		{
			name: "initialization with control plane config",
			nutanixConfig: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixNodeSpec{},
				},
			},
			workerNodeConfigs:          nil,
			expectedCheckCount:         4, // config check, credentials check, 1 VM image check, 1 storage container check
			expectedFirstCheckName:     "NutanixConfiguration",
			expectedSecondCheckName:    "NutanixCredentials",
			vmImageCheckCount:          1,
			storageContainerCheckCount: 1,
		},
		{
			name:          "initialization with worker node configs",
			nutanixConfig: nil,
			workerNodeConfigs: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixNodeSpec{},
				},
				"worker-2": {
					Nutanix: &carenv1.NutanixNodeSpec{},
				},
			},
			expectedCheckCount:         6, // config check, credentials check, 2 VM image checks, 2 storage container checks
			expectedFirstCheckName:     "NutanixConfiguration",
			expectedSecondCheckName:    "NutanixCredentials",
			vmImageCheckCount:          2,
			storageContainerCheckCount: 2,
		},
		{
			name: "initialization with both control plane and worker node configs",
			nutanixConfig: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixNodeSpec{},
				},
			},
			workerNodeConfigs: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixNodeSpec{},
				},
			},
			// config check, credentials check, 2 VM image checks (1 CP + 1 worker), 2 storage container checks (1 CP + 1 worker)
			expectedCheckCount:         6,
			expectedFirstCheckName:     "NutanixConfiguration",
			expectedSecondCheckName:    "NutanixCredentials",
			vmImageCheckCount:          2,
			storageContainerCheckCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the checker
			checker := &nutanixChecker{}

			// Mock the sub-check functions to track their calls
			configCheckCalled := false
			credsCheckCalled := false
			vmImageCheckCount := 0
			storageContainerCheckCount := 0

			checker.configurationCheckFactory = func(cd *checkDependencies) preflight.Check {
				configCheckCalled = true
				return &mockCheck{
					name:   tt.expectedFirstCheckName,
					result: preflight.CheckResult{Allowed: true},
				}
			}

			checker.credentialsCheckFactory = func(
				ctx context.Context,
				nclientFactory func(prismgoclient.Credentials) (client, error),
				cd *checkDependencies,
			) preflight.Check {
				credsCheckCalled = true
				return &mockCheck{
					name:   tt.expectedSecondCheckName,
					result: preflight.CheckResult{Allowed: true},
				}
			}

			checker.vmImageChecksFactory = func(cd *checkDependencies) []preflight.Check {
				checks := []preflight.Check{}
				for i := 0; i < tt.vmImageCheckCount; i++ {
					vmImageCheckCount++
					checks = append(checks,
						&mockCheck{
							name: fmt.Sprintf("NutanixVMImage-%d", i),
							result: preflight.CheckResult{
								Allowed: true,
							},
						},
					)
				}
				return checks
			}

			checker.initStorageContainerChecksFunc = func(n *nutanixChecker) []preflight.Check {
				checks := []preflight.Check{}
				for i := 0; i < tt.storageContainerCheckCount; i++ {
					storageContainerCheckCount++
					checks = append(checks, func(ctx context.Context) preflight.CheckResult {
						return preflight.CheckResult{
							Name: fmt.Sprintf("StorageContainerCheck-%d", i),
						}
					})
				}
				return checks
			}

			// Call Init
			ctx := context.Background()
			checks := checker.Init(ctx, nil, &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			})

			// Verify correct number of checks
			assert.Len(t, checks, tt.expectedCheckCount)

			// Verify the sub-functions were called
			assert.True(t, configCheckCalled, "initNutanixConfiguration should have been called")
			assert.True(t, credsCheckCalled, "initCredentialsCheck should have been called")
			assert.Equal(t, tt.vmImageCheckCount, vmImageCheckCount, "Wrong number of VM image checks")
			assert.Equal(
				t,
				tt.storageContainerCheckCount,
				storageContainerCheckCount,
				"Wrong number of storage container checks",
			)

			// Verify the first two checks when we have results
			if len(checks) >= 2 {
				assert.Equal(t, tt.expectedFirstCheckName, checks[0].Name())
				assert.Equal(t, tt.expectedSecondCheckName, checks[1].Name())
			}
		})
	}
}
