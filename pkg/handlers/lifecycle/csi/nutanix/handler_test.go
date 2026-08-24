// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

func TestStorageClassParameters(t *testing.T) {
	t.Run("keeps default class name and adds site-affinity variant for metro", func(t *testing.T) {
		original := map[string]v1alpha1.StorageClassConfig{
			"volume": {
				Parameters: map[string]string{
					"storageContainer": "sc1",
				},
			},
			"fast": {},
		}
		metroConfigs := metroStorageClassConfigs(original, "volume")

		if got, want := metroConfigs["volume"].Parameters[computeAffinityParameterKey], computeAffinityDisabled; got != want {
			t.Fatalf("expected default volume config to set computeAffinity to disabled, got %q", got)
		}
		if got := metroConfigs["fast"].Parameters[computeAffinityParameterKey]; got != "" {
			t.Fatalf("expected non-default fast config to remain unchanged, got computeAffinity %q", got)
		}
		siteName := metroStorageClassName("volume")
		siteConfig, exists := metroConfigs[siteName]
		if !exists {
			t.Fatalf("expected site-affinity class %q", siteName)
		}
		if got := siteConfig.Parameters[computeAffinityParameterKey]; got != "" {
			t.Fatalf("expected site-affinity class to omit computeAffinity, got %q", got)
		}
		if got, want := siteConfig.Parameters["storageContainer"], "sc1"; got != want {
			t.Fatalf("expected site-affinity class to copy storageContainer %q, got %q", want, got)
		}
		if _, exists := original["volume"].Parameters[computeAffinityParameterKey]; exists {
			t.Fatalf("did not expect original config to be mutated")
		}
	})

	t.Run("site-affinity omits computeAffinity even when default config already has it", func(t *testing.T) {
		original := map[string]v1alpha1.StorageClassConfig{
			"volume": {
				Parameters: map[string]string{
					computeAffinityParameterKey: computeAffinityDisabled,
					"storageContainer":          "sc1",
				},
			},
		}

		metroConfigs := metroStorageClassConfigs(original, "volume")

		if got := metroConfigs[metroStorageClassName("volume")].Parameters[computeAffinityParameterKey]; got != "" {
			t.Fatalf("expected site-affinity class to omit computeAffinity, got %q", got)
		}
		if got, want := original["volume"].Parameters[computeAffinityParameterKey], computeAffinityDisabled; got != want {
			t.Fatalf("did not expect original config to be mutated, got %q", got)
		}
	})

	t.Run("does not add site-affinity class when default class key is missing", func(t *testing.T) {
		original := map[string]v1alpha1.StorageClassConfig{
			"fast": {},
		}

		metroConfigs := metroStorageClassConfigs(original, "volume")

		if _, exists := metroConfigs["volume"+metroStorageClassSuffix]; exists {
			t.Fatalf("did not expect site-affinity class when default key is absent")
		}
		if got := metroConfigs["fast"].Parameters[computeAffinityParameterKey]; got != "" {
			t.Fatalf("expected existing config to remain unchanged, got computeAffinity %q", got)
		}
	})
}

func TestIsMetroCluster(t *testing.T) {
	tests := []struct {
		name    string
		cluster *clusterv1.Cluster
		want    bool
	}{
		{
			name: "metro control plane",
			cluster: clusterWithTopologyVariableAndWorkers(
				t,
				"mgmt",
				"default",
				[]string{metroFailureDomainPrefix + "metro-1"},
				nil,
			),
			want: true,
		},
		{
			name: "metro site on worker",
			cluster: clusterWithTopologyVariableAndWorkers(
				t,
				"mgmt",
				"default",
				nil,
				[]clusterv1.MachineDeploymentTopology{
					{Name: "md-1", FailureDomain: metroSiteFailureDomainPrefix + "site-1"},
				},
			),
			want: true,
		},
		{
			name: "metro workload cluster",
			cluster: clusterWithTopologyVariableAndWorkers(
				t,
				"workload",
				"default",
				[]string{metroFailureDomainPrefix + "metro-1"},
				nil,
			),
			want: true,
		},
		{
			name: "non metro cluster",
			cluster: clusterWithTopologyVariableAndWorkers(
				t,
				"mgmt",
				"default",
				[]string{"plain-fd"},
				nil,
			),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMetroCluster(tt.cluster)
			if got != tt.want {
				t.Fatalf("isMetroCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func clusterWithTopologyVariableAndWorkers(
	t *testing.T,
	name string,
	namespace string,
	controlPlaneFailureDomains []string,
	machineDeployments []clusterv1.MachineDeploymentTopology,
) *clusterv1.Cluster {
	t.Helper()

	clusterConfigVariable, err := apivariables.MarshalToClusterVariable(
		v1alpha1.ClusterConfigVariableName,
		apivariables.ClusterConfigSpec{
			ControlPlane: &apivariables.ControlPlaneSpec{
				Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
					FailureDomains: controlPlaneFailureDomains,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("failed to marshal cluster config variable: %v", err)
	}

	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				Variables: []clusterv1.ClusterVariable{*clusterConfigVariable},
				Workers: clusterv1.WorkersTopology{
					MachineDeployments: machineDeployments,
				},
			},
		},
	}
}
