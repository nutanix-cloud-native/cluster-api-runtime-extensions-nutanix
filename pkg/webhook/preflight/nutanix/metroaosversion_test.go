// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestClusterReferencesMetro(t *testing.T) {
	t.Parallel()

	assert.False(t, clusterReferencesMetro(nil))
	assert.False(t, clusterReferencesMetro(&checkDependencies{
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
					FailureDomains: []string{"plain-fd"},
				},
			},
		},
	}))
	assert.True(t, clusterReferencesMetro(&checkDependencies{
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
					FailureDomains: []string{metroFailureDomainPrefix + metroName},
				},
			},
		},
	}))
	assert.True(t, clusterReferencesMetro(&checkDependencies{
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				Topology: clusterv1beta2.Topology{
					Workers: clusterv1beta2.WorkersTopology{
						MachineDeployments: []clusterv1beta2.MachineDeploymentTopology{{
							Name:          "md-1",
							FailureDomain: metroSiteFailureDomainPrefix + "site-1",
						}},
					},
				},
			},
		},
	}))
}

func TestNewMetroAOSVersionChecks_GatedOnMetroFailureDomain(t *testing.T) {
	t.Parallel()

	cd := &checkDependencies{
		nclient:   &clientWrapper{},
		pcVersion: "7.5",
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
					FailureDomains: []string{"plain-fd"},
				},
			},
		},
		cluster: &clusterv1beta2.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
		},
		log: logr.Discard(),
	}
	assert.Nil(t, newMetroAOSVersionChecks(cd))
}

func TestNewMetroAOSVersionChecks_AddsOneCheckPerPrismElement(t *testing.T) {
	t.Parallel()

	kclient := fake.NewClientBuilder().WithScheme(metroScheme()).WithObjects(
		newMetroObject(metroName, metroFD1, metroFD2),
		newMetroFailureDomain(metroFD1, metroPE1UUID, metroSubnet1),
		newMetroFailureDomain(metroFD2, metroPE2UUID, metroSubnet2),
	).Build()

	cd := &checkDependencies{
		kclient:   kclient,
		nclient:   &clientWrapper{},
		pcVersion: "7.5",
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
					FailureDomains: []string{metroFailureDomainPrefix + metroName},
				},
			},
		},
		cluster: &clusterv1beta2.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
		},
		log: logr.Discard(),
	}

	checks := newMetroAOSVersionChecks(cd)
	require.Len(t, checks, 2)
	for _, check := range checks {
		assert.Equal(t, metroAOSVersionCheckName, check.Name())
	}
}

func TestMetroAOSVersionCheck_Run(t *testing.T) {
	t.Parallel()

	field := "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.machineDetails.cluster"

	tests := []struct {
		name          string
		version       string
		fullVersion   string
		expectAllowed bool
		expectCause   string
	}{
		{
			name:          "equal to metro floor passes",
			version:       "7.5.1",
			expectAllowed: true,
		},
		{
			name:          "patch above metro floor passes",
			version:       "7.5.2",
			expectAllowed: true,
		},
		{
			name:          "above metro floor passes",
			version:       "7.6",
			expectAllowed: true,
		},
		{
			name:          "minor.patch above metro floor passes",
			version:       "7.6.1",
			expectAllowed: true,
		},
		{
			name:          "generic 7.5 floor is too old for metro",
			version:       "7.5",
			expectAllowed: false,
			expectCause:   "minimum supported version " + minMetroAOSVersion,
		},
		{
			name:          "truncated short 7.5 with fullVersion 7.5.1 passes",
			version:       "7.5",
			fullVersion:   "el8.5-release-ganges-7.5.1-stable-b2a0591e15d312a5888e2eee14e1c4bffc3e3b3d",
			expectAllowed: true,
		},
		{
			name:          "truncated short 7.5 with fullVersion also 7.5 fails",
			version:       "7.5",
			fullVersion:   "el8.5-release-ganges-7.5-stable-abc123",
			expectAllowed: false,
			expectCause:   "minimum supported version " + minMetroAOSVersion,
		},
		{
			name:          "internal build passes",
			version:       "master",
			expectAllowed: true,
		},
		{
			name:          "internal build with opt-master fullVersion passes",
			version:       "master",
			fullVersion:   "el9-opt-master-56c4af6e0fb9c6a2aed0e517cbfb3768cde74311",
			expectAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			check := &aosVersionCheck{
				machineSpec: machineDetailsWithCluster("pe-uuid"),
				field:       field,
				name:        metroAOSVersionCheckName,
				nclient: &clientWrapper{
					GetClusterByIdFunc: clusterByIDBuildInfoFunc(t, tt.version, tt.fullVersion),
				},
				minVersion: minMetroAOSVersion,
				pcVersion:  "7.5",
				metroOnly:  true,
			}

			result := check.Run(context.Background())
			assert.Equal(t, tt.expectAllowed, result.Allowed)
			assert.Equal(t, metroAOSVersionCheckName, check.Name())
			if tt.expectCause != "" {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, tt.expectCause)
				assert.Contains(t, result.Causes[0].Message, "metro")
			}
		})
	}
}
