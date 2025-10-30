// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Test_isCloudProviderSupported(t *testing.T) {
	tests := []struct {
		name           string
		cluster        *clusterv1.Cluster
		expectedResult bool
		expectedName   string
	}{
		{
			name: "EKS cluster is supported",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSManagedCluster",
					},
				},
			},
			expectedResult: true,
			expectedName:   "EKS",
		},
		{
			name: "Nutanix cluster is supported",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "NutanixCluster",
					},
				},
			},
			expectedResult: true,
			expectedName:   "Nutanix",
		},
		{
			name: "Unsupported provider returns false",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "DockerCluster",
					},
				},
			},
			expectedResult: false,
			expectedName:   "",
		},
		{
			name: "Nil infrastructure reference returns false",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: nil,
				},
			},
			expectedResult: false,
			expectedName:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployer := &MultusDeployer{}
			isSupported, providerName := deployer.isCloudProviderSupported(tt.cluster)

			assert.Equal(t, tt.expectedResult, isSupported)
			assert.Equal(t, tt.expectedName, providerName)
		})
	}
}
