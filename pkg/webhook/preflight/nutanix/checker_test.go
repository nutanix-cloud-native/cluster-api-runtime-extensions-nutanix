// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestNutanixChecker_Init(t *testing.T) {
	tests := []struct {
		name                    string
		nutanixConfig           *carenv1.NutanixClusterConfigSpec
		workerNodeConfigs       map[string]*carenv1.NutanixWorkerNodeConfigSpec
		expectedCheckCount      int
		expectedFirstCheckName  string
		expectedSecondCheckName string
		vmImageCheckCount       int
	}{
		{
			name:                    "basic initialization with no configs",
			nutanixConfig:           nil,
			workerNodeConfigs:       nil,
			expectedCheckCount:      2, // config check and credentials check
			expectedFirstCheckName:  "NutanixConfiguration",
			expectedSecondCheckName: "NutanixCredentials",
			vmImageCheckCount:       0,
		},
		{
			name: "initialization with control plane config",
			nutanixConfig: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixNodeSpec{},
				},
			},
			workerNodeConfigs:       nil,
			expectedCheckCount:      3, // config check, credentials check, 1 VM image check
			expectedFirstCheckName:  "NutanixConfiguration",
			expectedSecondCheckName: "NutanixCredentials",
			vmImageCheckCount:       1,
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
			expectedCheckCount:      4, // config check, credentials check, 2 VM image checks
			expectedFirstCheckName:  "NutanixConfiguration",
			expectedSecondCheckName: "NutanixCredentials",
			vmImageCheckCount:       2,
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
			expectedCheckCount:      4, // config check, credentials check, 2 VM image checks (1 CP + 1 worker)
			expectedFirstCheckName:  "NutanixConfiguration",
			expectedSecondCheckName: "NutanixCredentials",
			vmImageCheckCount:       2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the checker
			checker := &nutanixChecker{
				cluster:                  &clusterv1.Cluster{},
				nutanixClusterConfigSpec: tt.nutanixConfig,
				nutanixWorkerNodeConfigSpecByMachineDeploymentName: tt.workerNodeConfigs,
			}

			// Mock the sub-check functions to track their calls
			configCheckCalled := false
			credsCheckCalled := false
			vmImageCheckCount := 0

			checker.initNutanixConfigurationFunc = func(n *nutanixChecker) preflight.Check {
				configCheckCalled = true
				return func(ctx context.Context) preflight.CheckResult {
					return preflight.CheckResult{
						Name: tt.expectedFirstCheckName,
					}
				}
			}

			checker.initCredentialsCheckFunc = func(ctx context.Context, n *nutanixChecker) preflight.Check {
				credsCheckCalled = true
				return func(ctx context.Context) preflight.CheckResult {
					return preflight.CheckResult{
						Name: tt.expectedSecondCheckName,
					}
				}
			}

			checker.initVMImageChecksFunc = func(n *nutanixChecker) []preflight.Check {
				checks := []preflight.Check{}
				for i := 0; i < tt.vmImageCheckCount; i++ {
					vmImageCheckCount++
					checks = append(checks, func(ctx context.Context) preflight.CheckResult {
						return preflight.CheckResult{
							Name: fmt.Sprintf("VMImageCheck-%d", i),
						}
					})
				}
				return checks
			}

			// Call Init
			ctx := context.Background()
			checks := checker.Init(ctx)

			// Verify the logger was set
			assert.NotNil(t, checker.log)

			// Verify correct number of checks
			assert.Len(t, checks, tt.expectedCheckCount)

			// Verify the sub-functions were called
			assert.True(t, configCheckCalled, "initNutanixConfiguration should have been called")
			assert.True(t, credsCheckCalled, "initCredentialsCheck should have been called")
			assert.Equal(t, tt.vmImageCheckCount, vmImageCheckCount, "Wrong number of VM image checks")

			// Verify the first two checks when we have results
			if len(checks) >= 2 {
				result1 := checks[0](ctx)
				result2 := checks[1](ctx)
				assert.Equal(t, tt.expectedFirstCheckName, result1.Name)
				assert.Equal(t, tt.expectedSecondCheckName, result2.Name)
			}
		})
	}
}
