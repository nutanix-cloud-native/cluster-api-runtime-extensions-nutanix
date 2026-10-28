// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

// csiCheckDeps builds check dependencies for the Nutanix CSI project checks.
func csiCheckDeps(
	policy string,
	storagePolicy string,
	scParameters map[string]string,
	controlPlaneProject *capxv1.NutanixResourceIdentifier,
	defaultProjectUUID string,
) *checkDependencies {
	annotations := map[string]string{}
	if policy != "" {
		annotations["capx.nutanix.com/project-policy"] = policy
	}
	if storagePolicy != "" {
		annotations["csi.nutanix.com/storage-project-policy"] = storagePolicy
	}

	spec := &carenv1.NutanixClusterConfigSpec{}
	if scParameters != nil {
		spec.Addons = &carenv1.NutanixAddons{
			CSI: &carenv1.NutanixCSI{
				Providers: carenv1.NutanixCSIProviders{
					NutanixCSI: carenv1.CSIProvider{
						StorageClassConfigs: map[string]carenv1.StorageClassConfig{
							"nutanix": {Parameters: scParameters},
						},
					},
				},
			},
		}
	}
	if controlPlaneProject != nil {
		spec.ControlPlane = &carenv1.NutanixControlPlaneSpec{
			Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Project: controlPlaneProject,
				},
			},
		}
	}

	return &checkDependencies{
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "c",
				Namespace:   "ns",
				Annotations: annotations,
			},
		},
		nutanixClusterConfigSpec: spec,
		log:                      logr.Discard(),
		nclient: &clientWrapper{
			ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
				return &PCDefaultProject{UUID: defaultProjectUUID, Name: "default"}, nil
			},
		},
	}
}

func runCSIChecks(cd *checkDependencies) (checkCount int, allowed bool, causes int) {
	checks := newCSIChecks(cd)
	allowed = true
	for _, chk := range checks {
		res := chk.Run(context.Background())
		if !res.Allowed {
			allowed = false
		}
		causes += len(res.Causes)
	}
	return len(checks), allowed, causes
}

func TestCSIChecks(t *testing.T) {
	t.Parallel()

	defaultProject := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("def-uuid"),
	}
	nonDefaultProject := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("other-uuid"),
	}

	tests := []struct {
		name               string
		policy             string
		storagePolicy      string
		scParameters       map[string]string
		controlPlaneProj   *capxv1.NutanixResourceIdentifier
		expectedCheckCount int
		expectedAllowed    bool
	}{
		{
			name:               "no CSI storage class config, no checks",
			policy:             "single-project",
			scParameters:       nil,
			expectedCheckCount: 0,
			expectedAllowed:    true,
		},
		{
			name:               "hypervisorAttached ENABLED, no checks",
			policy:             "single-project",
			scParameters:       map[string]string{"hypervisorAttached": "ENABLED"},
			expectedCheckCount: 0,
			expectedAllowed:    true,
		},
		{
			name:               "hypervisorAttached absent defaults to DISABLED, checks run and deny non-default policy",
			policy:             "single-project",
			scParameters:       map[string]string{"storageType": "NutanixVolumes"},
			expectedCheckCount: 2,
			expectedAllowed:    false,
		},
		{
			name:               "hypervisorAttached absent defaults to DISABLED, default-only policy and no refs allowed",
			policy:             "default-only",
			scParameters:       map[string]string{"storageType": "NutanixVolumes"},
			expectedCheckCount: 2,
			expectedAllowed:    true,
		},
		{
			name:               "hypervisorAttached DISABLED with default-only policy and no refs, allowed",
			policy:             "default-only",
			scParameters:       map[string]string{"hypervisorAttached": "DISABLED"},
			expectedCheckCount: 2,
			expectedAllowed:    true,
		},
		{
			name:               "hypervisorAttached DISABLED with single-project policy, denied",
			policy:             "single-project",
			scParameters:       map[string]string{"hypervisorAttached": "DISABLED"},
			expectedCheckCount: 2,
			expectedAllowed:    false,
		},
		{
			name:               "hypervisorAttached DISABLED with unrestricted policy, denied",
			policy:             "unrestricted",
			scParameters:       map[string]string{"hypervisorAttached": "DISABLED"},
			expectedCheckCount: 2,
			expectedAllowed:    false,
		},
		{
			name:               "hypervisorAttached DISABLED with default-only and non-default project ref, denied",
			policy:             "default-only",
			scParameters:       map[string]string{"hypervisorAttached": "DISABLED"},
			controlPlaneProj:   nonDefaultProject,
			expectedCheckCount: 2,
			expectedAllowed:    false,
		},
		{
			name:               "hypervisorAttached DISABLED with default-only and default project ref, allowed",
			policy:             "default-only",
			scParameters:       map[string]string{"hypervisorAttached": "DISABLED"},
			controlPlaneProj:   defaultProject,
			expectedCheckCount: 2,
			expectedAllowed:    true,
		},
		{
			name:               "hypervisorAttached disabled case-insensitive with unrestricted policy, denied",
			policy:             "unrestricted",
			scParameters:       map[string]string{"hypervisorAttached": "disabled"},
			expectedCheckCount: 2,
			expectedAllowed:    false,
		},
		{
			name:               "storage-project-policy default skips CSI checks even with single-project",
			policy:             "single-project",
			storagePolicy:      "default",
			scParameters:       map[string]string{"storageType": "NutanixVolumes"},
			controlPlaneProj:   nonDefaultProject,
			expectedCheckCount: 0,
			expectedAllowed:    true,
		},
		{
			name:               "storage-project-policy project-policy still runs CSI checks",
			policy:             "single-project",
			storagePolicy:      "project-policy",
			scParameters:       map[string]string{"hypervisorAttached": "DISABLED"},
			expectedCheckCount: 2,
			expectedAllowed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cd := csiCheckDeps(tt.policy, tt.storagePolicy, tt.scParameters, tt.controlPlaneProj, "def-uuid")
			checkCount, allowed, causes := runCSIChecks(cd)

			assert.Equal(t, tt.expectedCheckCount, checkCount, "unexpected number of checks")
			assert.Equal(t, tt.expectedAllowed, allowed, "unexpected allowed result")
			if !tt.expectedAllowed {
				assert.NotZero(t, causes, "expected at least one cause when denied")
			}
		})
	}
}
