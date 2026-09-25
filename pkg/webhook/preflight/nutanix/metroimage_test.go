// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestMatchedUnsupportedOS(t *testing.T) {
	t.Parallel()

	version, ok := matchedUnsupportedOS("rhel-8.10")
	assert.True(t, ok)
	assert.Equal(t, "rhel-8.10", version)

	version, ok = matchedUnsupportedOS("RHEL-8.10")
	assert.True(t, ok)
	assert.Equal(t, "rhel-8.10", version)

	version, ok = matchedUnsupportedOS("nkp-rhel-8.10-release-1.33.1-20250704023459")
	assert.True(t, ok)
	assert.Equal(t, "rhel-8.10", version)

	_, ok = matchedUnsupportedOS("rhel-8.1")
	assert.False(t, ok)
	_, ok = matchedUnsupportedOS("rhel-8.100")
	assert.False(t, ok)
	_, ok = matchedUnsupportedOS("rhel-9.4")
	assert.False(t, ok)
	_, ok = matchedUnsupportedOS("rocky-9.6")
	assert.False(t, ok)
	_, ok = matchedUnsupportedOS("")
	assert.False(t, ok)
}

func TestMatchUnsupportedOS_AdditionalVersionUsesSameMatcher(t *testing.T) {
	t.Parallel()

	patterns := unsupportedOSPatterns([]string{"rhel-8.10", "rhel-8.13"})

	version, ok := matchUnsupportedOS("nkp-rhel-8.13-release-1.33.1", patterns)
	assert.True(t, ok)
	assert.Equal(t, "rhel-8.13", version)

	version, ok = matchUnsupportedOS("nkp-rhel-8.10-release-1.33.1", patterns)
	assert.True(t, ok)
	assert.Equal(t, "rhel-8.10", version)

	_, ok = matchUnsupportedOS("nkp-rhel-8.1-release-1.33.1", patterns)
	assert.False(t, ok)
}

func TestNewMetroVMImageChecks_GatedOnMetro(t *testing.T) {
	t.Parallel()

	nonMetro := &checkDependencies{
		nclient:   &clientWrapper{},
		pcVersion: "7.5",
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
					FailureDomains: []string{"plain-fd"},
					MachineDetails: rockyImageLookup(),
				},
			},
		},
	}
	assert.Nil(t, newMetroVMImageChecks(nonMetro))
	assert.Nil(t, newMetroVMImageChecks(nil))
	assert.Nil(t, newMetroVMImageChecks(&checkDependencies{}))
}

func TestNewMetroVMImageChecks_OneCheckPerNodePool(t *testing.T) {
	t.Parallel()

	cd := &checkDependencies{
		nclient:   &clientWrapper{},
		pcVersion: "7.5",
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
					FailureDomains: []string{metroFailureDomainPrefix + metroName},
					MachineDetails: rockyImageLookup(),
				},
			},
		},
		nutanixWorkerNodeConfigSpecByMachineDeploymentName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
			"md-1": {
				Nutanix: &carenv1.NutanixWorkerNodeSpec{
					MachineDetails: rockyImageLookup(),
				},
			},
		},
	}

	checks := newMetroVMImageChecks(cd)
	require.Len(t, checks, 2)
	for _, check := range checks {
		assert.Equal(t, metroVMImageCheckName, check.Name())
	}
}

func TestMetroVMImageCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		machineDetails *carenv1.NutanixMachineDetails
		nclient        client
		expectAllowed  bool
		expectInternal bool
		expectCause    string
	}{
		{
			name:           "rhel 8.10 image is rejected",
			machineDetails: imageDetails("test-uuid"),
			nclient:        imageByIDClient(t, "nkp-rhel-8.10-release-1.33.1"),
			expectAllowed:  false,
			expectCause:    `Metro clusters do not support rhel-8.10. The Control Plane uses VM image "test-uuid", named "nkp-rhel-8.10-release-1.33.1" in Prism Central.`,
		},
		{
			name:           "rocky image passes",
			machineDetails: imageDetails("test-uuid"),
			nclient:        imageByIDClient(t, "nkp-rocky-9.6-release-1.33.1"),
			expectAllowed:  true,
		},
		{
			name:           "rhel 9 image passes",
			machineDetails: imageDetails("test-uuid"),
			nclient:        imageByIDClient(t, "nkp-rhel-9.4-release-1.33.1"),
			expectAllowed:  true,
		},
		{
			name:           "rhel 8.1 image passes",
			machineDetails: imageDetails("test-uuid"),
			nclient:        imageByIDClient(t, "nkp-rhel-8.1-release-1.33.1"),
			expectAllowed:  true,
		},
		{
			name: "imageLookup rhel 8.10 is rejected",
			machineDetails: &carenv1.NutanixMachineDetails{
				ImageLookup: &capxv1.NutanixImageLookup{BaseOS: "rhel-8.10"},
			},
			nclient:       &clientWrapper{},
			expectAllowed: false,
			expectCause:   "Metro clusters do not support rhel-8.10. The Control Plane sets imageLookup.baseOS to \"rhel-8.10\".",
		},
		{
			name: "imageLookup rocky passes",
			machineDetails: &carenv1.NutanixMachineDetails{
				ImageLookup: &capxv1.NutanixImageLookup{BaseOS: "rocky-9.6"},
			},
			nclient:       &clientWrapper{},
			expectAllowed: true,
		},
		{
			name:           "missing image is not an unsupported OS violation",
			machineDetails: &carenv1.NutanixMachineDetails{},
			nclient:        &clientWrapper{},
			expectAllowed:  true,
		},
		{
			name:           "prism error fails closed",
			machineDetails: imageDetails("test-uuid"),
			nclient: &clientWrapper{
				GetImageByIdFunc: func(context.Context, *string, ...map[string]any) (*vmmv4.GetImageApiResponse, error) {
					return nil, fmt.Errorf("api error")
				},
			},
			expectAllowed:  false,
			expectInternal: true,
			expectCause:    "temporary error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			check := &metroVMImageCheck{
				description:    "The Control Plane",
				machineDetails: tt.machineDetails,
				field:          controlPlaneMachineDetailsField,
				nclient:        tt.nclient,
			}

			result := check.Run(context.Background())
			assert.Equal(t, tt.expectAllowed, result.Allowed)
			assert.Equal(t, tt.expectInternal, result.InternalError)
			assert.Equal(t, metroVMImageCheckName, check.Name())
			if tt.expectCause != "" {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, tt.expectCause)
			} else {
				assert.Empty(t, result.Causes)
			}
		})
	}
}

func TestMetroVMImageSubject(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		`The Control Plane uses VM image "nkp-rhel-8.10-release-1.36.2".`,
		metroVMImageSubject("The Control Plane", "nkp-rhel-8.10-release-1.36.2", "nkp-rhel-8.10-release-1.36.2"),
	)
	assert.Equal(t,
		`The Control Plane uses VM image "test-uuid", named "nkp-rhel-8.10-release-1.33.1" in Prism Central.`,
		metroVMImageSubject("The Control Plane", "test-uuid", "nkp-rhel-8.10-release-1.33.1"),
	)
}

func rockyImageLookup() carenv1.NutanixMachineDetails {
	return carenv1.NutanixMachineDetails{
		ImageLookup: &capxv1.NutanixImageLookup{BaseOS: "rocky-9.6"},
	}
}

func imageDetails(uuid string) *carenv1.NutanixMachineDetails {
	return &carenv1.NutanixMachineDetails{
		Image: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierUUID,
			UUID: ptr.To(uuid),
		},
	}
}

func imageByIDClient(t *testing.T, name string) client {
	t.Helper()
	return &clientWrapper{
		GetImageByIdFunc: func(_ context.Context, uuid *string, _ ...map[string]any) (*vmmv4.GetImageApiResponse, error) {
			require.NotNil(t, uuid)
			resp := &vmmv4.GetImageApiResponse{}
			image := vmmv4.Image{
				ObjectType_: ptr.To("vmm.v4.content.Image"),
				ExtId:       uuid,
			}
			if name != "" {
				image.Name = ptr.To(name)
			}
			require.NoError(t, resp.SetData(image))
			return resp, nil
		},
	}
}
