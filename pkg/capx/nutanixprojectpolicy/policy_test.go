// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanixprojectpolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestPCVersionAtLeast76(t *testing.T) {
	t.Parallel()
	assert.True(t, PCVersionAtLeast76("7.6.0"), "expected 7.6.0 to satisfy")
	assert.False(t, PCVersionAtLeast76("7.5.99"), "expected 7.5.99 to not satisfy")
	assert.False(t, PCVersionAtLeast76("skipped"), "skipped must not satisfy")
	assert.True(t, PCVersionAtLeast76("master-2026"), "dev master label should satisfy")
}

func TestStorageProjectPolicyIsDefault(t *testing.T) {
	t.Parallel()

	assert.False(t, StorageProjectPolicyIsDefault(nil))
	assert.False(t, StorageProjectPolicyIsDefault(map[string]string{}))
	assert.False(t, StorageProjectPolicyIsDefault(map[string]string{
		CSIStorageProjectPolicyAnnotationKey: StoragePolicyProjectPolicy,
	}))
	assert.True(t, StorageProjectPolicyIsDefault(map[string]string{
		CSIStorageProjectPolicyAnnotationKey: StoragePolicyDefault,
	}))
	assert.True(t, StorageProjectPolicyIsDefault(map[string]string{
		CSIStorageProjectPolicyAnnotationKey: " default ",
	}))
	assert.True(t, StorageProjectPolicyIsDefault(map[string]string{
		CSIStorageProjectPolicyAnnotationKey: "Default",
	}))
	assert.True(t, StorageProjectPolicyIsDefault(map[string]string{
		CSIStorageProjectPolicyAnnotationKey: "DEFAULT",
	}))
	assert.False(t, StorageProjectPolicyIsProjectPolicy(nil))
	assert.True(t, StorageProjectPolicyIsProjectPolicy(map[string]string{
		CSIStorageProjectPolicyAnnotationKey: StoragePolicyProjectPolicy,
	}))
	assert.True(t, StorageProjectPolicyIsProjectPolicy(map[string]string{
		CSIStorageProjectPolicyAnnotationKey: "Project-Policy",
	}))
	assert.False(t, StorageProjectPolicyIsProjectPolicy(map[string]string{
		CSIStorageProjectPolicyAnnotationKey: StoragePolicyDefault,
	}))
}

func TestCollectReferencedProjects(t *testing.T) {
	t.Parallel()
	p := &capxv1.NutanixResourceIdentifier{Type: capxv1.NutanixIdentifierUUID, UUID: new("u")}
	spec := &carenv1.NutanixClusterConfigSpec{
		ControlPlane: &carenv1.NutanixControlPlaneSpec{
			Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{Project: p},
			},
		},
	}
	workers := map[string]*carenv1.NutanixWorkerNodeConfigSpec{
		"w": {
			Nutanix: &carenv1.NutanixWorkerNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Project: &capxv1.NutanixResourceIdentifier{Type: capxv1.NutanixIdentifierUUID, UUID: new("w")},
				},
			},
		},
	}
	refs := CollectReferencedProjects(spec, workers)
	require.Len(t, refs, 2)
}
