// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package generic

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestNewConfigurationCheck(t *testing.T) {
	tests := []struct {
		name                             string
		cluster                          *clusterv1.Cluster
		expectedResult                   preflight.CheckResult
		expectedGenericClusterConfigSpec bool
	}{
		{
			name: "global image registry mirror config",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{
										"globalImageRegistryMirror": {
											"credentials": {
												"secretRef": {
													"name": "nai-demo-image-registry-mirror-credentials"
												}
											},
											"url": "https://artifactory.canaveral-corp.us-west-2.aws"
										}
									}`),
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedGenericClusterConfigSpec: true,
		},
		{
			name: "multiple image registries config",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{
										"imageRegistries": [
											{
												"url": "https://my-registry.io",
												"credentials": {
													"secretRef": {
														"name": "my-registry-credentials"
													}
												}
											}
										]
									}`),
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedGenericClusterConfigSpec: true,
		},
		{
			name: "invalid cluster config json",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{invalid-json`),
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "Failed to unmarshal cluster variable clusterConfig: failed to unmarshal json: invalid character 'i' looking for beginning of object key string",
						Field:   "cluster.spec.topology.variables[.name=clusterConfig].genericClusterConfigSpec",
					},
				},
			},
			expectedGenericClusterConfigSpec: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cd := &checkDependencies{
				cluster: tt.cluster,
				log:     testr.New(t),
			}

			check := newConfigurationCheck(cd)
			result := check.Run(context.Background())

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
