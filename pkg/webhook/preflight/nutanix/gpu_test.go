// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	clustermgmtv4ahv "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/ahv/config"
	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	multidomainv4 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/prism-go-client/converged"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestGPUCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		client    *clientWrapper
		gpu       capxv1.NutanixGPU
		pcVersion string
		wantOK    bool
		wantErr   bool
		// wantMessage pins the user-facing cause message. A profile-typed entry must be described as
		// a "GPU Profile", never as a "GPU with profile ...".
		wantMessage string
	}{
		{
			name: "device exists by name on selected Prism Element",
			client: &clientWrapper{
				ListClusterPhysicalGPUsFunc: func(_ context.Context, peUUID string) ([]clustermgmtv4.PhysicalGpuProfile, error) {
					assert.Equal(t, "pe-uuid", peUUID)
					return []clustermgmtv4.PhysicalGpuProfile{{
						PhysicalGpuConfig: &clustermgmtv4.PhysicalGpuConfig{DeviceName: new("NVIDIA A100")},
					}}, nil
				},
				ListClusterVirtualGPUsFunc: func(context.Context, string) ([]clustermgmtv4.VirtualGpuProfile, error) {
					return nil, nil
				},
			},
			gpu:    capxv1.NutanixGPU{Type: capxv1.NutanixGPUIdentifierName, Name: new("nvidia a100")},
			wantOK: true,
		},
		{
			name: "device exists by device ID on selected Prism Element",
			client: &clientWrapper{
				ListClusterPhysicalGPUsFunc: func(_ context.Context, peUUID string) ([]clustermgmtv4.PhysicalGpuProfile, error) {
					assert.Equal(t, "pe-uuid", peUUID)
					return nil, nil
				},
				ListClusterVirtualGPUsFunc: func(_ context.Context, peUUID string) ([]clustermgmtv4.VirtualGpuProfile, error) {
					assert.Equal(t, "pe-uuid", peUUID)
					return []clustermgmtv4.VirtualGpuProfile{{
						VirtualGpuConfig: &clustermgmtv4.VirtualGpuConfig{DeviceId: new(int64(42))},
					}}, nil
				},
			},
			gpu:    capxv1.NutanixGPU{Type: capxv1.NutanixGPUIdentifierDeviceID, DeviceID: new(int64(42))},
			wantOK: true,
		},
		{
			name: "GPU profile exists on selected Prism Element",
			client: &clientWrapper{
				ListAHVPhysicalGPUProfilesFunc: func(_ context.Context, peUUID string) ([]clustermgmtv4ahv.PhysicalGpuProfile, error) {
					assert.Equal(t, "pe-uuid", peUUID)
					return []clustermgmtv4ahv.PhysicalGpuProfile{{
						Configuration: &clustermgmtv4ahv.PhysicalGpuProfileConfiguration{Name: new("A100-40C")},
						ExtId:         new("profile-uuid"),
					}}, nil
				},
				ListAHVVirtualGPUProfilesFunc: func(context.Context, string) ([]clustermgmtv4ahv.VirtualGpuProfile, error) {
					return nil, nil
				},
			},
			gpu: capxv1.NutanixGPU{
				Type: capxv1.NutanixGPUIdentifierProfile,
				Profile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("A100-40C"),
				},
			},
			pcVersion: "7.6",
			wantOK:    true,
		},
		{
			name: "GPU profile exists by UUID on selected Prism Element",
			client: &clientWrapper{
				ListAHVPhysicalGPUProfilesFunc: func(_ context.Context, peUUID string) ([]clustermgmtv4ahv.PhysicalGpuProfile, error) {
					assert.Equal(t, "pe-uuid", peUUID)
					return []clustermgmtv4ahv.PhysicalGpuProfile{{
						Configuration: &clustermgmtv4ahv.PhysicalGpuProfileConfiguration{Name: new("A100-40C")},
						ExtId:         new("profile-uuid"),
					}}, nil
				},
				ListAHVVirtualGPUProfilesFunc: func(context.Context, string) ([]clustermgmtv4ahv.VirtualGpuProfile, error) {
					return nil, nil
				},
			},
			gpu: capxv1.NutanixGPU{
				Type: capxv1.NutanixGPUIdentifierProfile,
				Profile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: new("profile-uuid"),
				},
			},
			pcVersion: "7.6",
			wantOK:    true,
		},
		{
			name: "GPU profile is absent from selected Prism Element",
			client: &clientWrapper{
				ListAHVPhysicalGPUProfilesFunc: func(context.Context, string) ([]clustermgmtv4ahv.PhysicalGpuProfile, error) {
					return nil, nil
				},
				ListAHVVirtualGPUProfilesFunc: func(context.Context, string) ([]clustermgmtv4ahv.VirtualGpuProfile, error) {
					return nil, nil
				},
			},
			gpu: capxv1.NutanixGPU{
				Type: capxv1.NutanixGPUIdentifierProfile,
				Profile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("missing"),
				},
			},
			pcVersion:   "7.6",
			wantOK:      false,
			wantMessage: `GPU Profile with name "missing" does not exist on Prism Element "pe-uuid".`,
		},
		{
			name: "GPU profile is ambiguous",
			client: &clientWrapper{
				ListAHVPhysicalGPUProfilesFunc: func(context.Context, string) ([]clustermgmtv4ahv.PhysicalGpuProfile, error) {
					return []clustermgmtv4ahv.PhysicalGpuProfile{{
						Configuration: &clustermgmtv4ahv.PhysicalGpuProfileConfiguration{Name: new("A100-40C")},
						ExtId:         new("physical-profile-uuid"),
					}}, nil
				},
				ListAHVVirtualGPUProfilesFunc: func(context.Context, string) ([]clustermgmtv4ahv.VirtualGpuProfile, error) {
					return []clustermgmtv4ahv.VirtualGpuProfile{{
						Configuration: &clustermgmtv4ahv.VirtualGpuProfileConfiguration{Name: new("A100-40C")},
						ExtId:         new("virtual-profile-uuid"),
					}}, nil
				},
			},
			gpu: capxv1.NutanixGPU{
				Type: capxv1.NutanixGPUIdentifierProfile,
				Profile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("A100-40C"),
				},
			},
			pcVersion:   "7.6",
			wantOK:      false,
			wantMessage: `GPU Profile with name "A100-40C" is ambiguous on the selected Prism Element.`,
		},
		{
			name: "GPU profile UUID without extId is absent",
			client: &clientWrapper{
				ListAHVPhysicalGPUProfilesFunc: func(context.Context, string) ([]clustermgmtv4ahv.PhysicalGpuProfile, error) {
					return []clustermgmtv4ahv.PhysicalGpuProfile{{
						Configuration: &clustermgmtv4ahv.PhysicalGpuProfileConfiguration{Name: new("A100-40C")},
					}}, nil
				},
				ListAHVVirtualGPUProfilesFunc: func(context.Context, string) ([]clustermgmtv4ahv.VirtualGpuProfile, error) {
					return nil, nil
				},
			},
			gpu: capxv1.NutanixGPU{
				Type: capxv1.NutanixGPUIdentifierProfile,
				Profile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: new("profile-uuid"),
				},
			},
			pcVersion:   "7.6",
			wantOK:      false,
			wantMessage: `GPU Profile with UUID "profile-uuid" does not exist on Prism Element "pe-uuid".`,
		},
		{
			name:   "GPU profile requires Prism Central 7.6",
			client: &clientWrapper{},
			gpu: capxv1.NutanixGPU{
				Type: capxv1.NutanixGPUIdentifierProfile,
				Profile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("A100-40C"),
				},
			},
			pcVersion:   "7.5",
			wantOK:      false,
			wantMessage: `GPU Profile with name "A100-40C" requires Prism Central 7.6 or later.`,
		},
		{
			name:   "GPU profile requires a name or UUID identifier",
			client: &clientWrapper{},
			gpu: capxv1.NutanixGPU{
				Type: capxv1.NutanixGPUIdentifierProfile,
			},
			pcVersion:   "7.6",
			wantOK:      false,
			wantMessage: "GPU Profile requires an identifier with a name or UUID.",
		},
		{
			name: "GPU device exists even when already in use",
			client: &clientWrapper{
				ListClusterPhysicalGPUsFunc: func(context.Context, string) ([]clustermgmtv4.PhysicalGpuProfile, error) {
					return []clustermgmtv4.PhysicalGpuProfile{{
						PhysicalGpuConfig: &clustermgmtv4.PhysicalGpuConfig{
							DeviceName: new("A100"),
							IsInUse:    new(true),
						},
					}}, nil
				},
				ListClusterVirtualGPUsFunc: func(context.Context, string) ([]clustermgmtv4.VirtualGpuProfile, error) {
					return nil, nil
				},
			},
			gpu:    capxv1.NutanixGPU{Type: capxv1.NutanixGPUIdentifierName, Name: new("A100")},
			wantOK: true,
		},
		{
			name: "GPU is absent from selected Prism Element",
			client: &clientWrapper{
				ListClusterPhysicalGPUsFunc: func(context.Context, string) ([]clustermgmtv4.PhysicalGpuProfile, error) {
					return nil, nil
				},
				ListClusterVirtualGPUsFunc: func(context.Context, string) ([]clustermgmtv4.VirtualGpuProfile, error) {
					return nil, nil
				},
			},
			gpu:         capxv1.NutanixGPU{Type: capxv1.NutanixGPUIdentifierName, Name: new("missing")},
			wantOK:      false,
			wantMessage: `GPU with name "missing" does not exist on Prism Element "pe-uuid".`,
		},
		{
			name: "GPU catalog API fails",
			client: &clientWrapper{
				ListClusterPhysicalGPUsFunc: func(context.Context, string) ([]clustermgmtv4.PhysicalGpuProfile, error) {
					return nil, fmt.Errorf("temporary API error")
				},
			},
			gpu:     capxv1.NutanixGPU{Type: capxv1.NutanixGPUIdentifierName, Name: new("A100")},
			wantOK:  false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.client.ResolvePCDefaultProjectFunc == nil {
				tt.client.ResolvePCDefaultProjectFunc = func(context.Context, string) (*PCDefaultProject, error) {
					return &PCDefaultProject{UUID: "default-project"}, nil
				}
			}
			check := &gpuCheck{
				machineDetails: &carenv1.NutanixMachineDetails{
					Cluster: &capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierUUID,
						UUID: new("pe-uuid"),
					},
					GPUs: []capxv1.NutanixGPU{tt.gpu},
				},
				field:     "test-field",
				nclient:   tt.client,
				pcVersion: tt.pcVersion,
			}

			result := check.Run(context.Background())
			assert.Equal(t, tt.wantOK, result.Allowed)
			assert.Equal(t, tt.wantErr, result.InternalError)
			if !tt.wantOK {
				require.Len(t, result.Causes, 1)
				assert.Equal(t, "test-field.gpus[0]", result.Causes[0].Field)
				if tt.wantMessage != "" {
					assert.Equal(t, tt.wantMessage, result.Causes[0].Message)
				}
			}
		})
	}
}

func TestGPUCheckProjectScopedPE(t *testing.T) {
	t.Parallel()

	client := &clientWrapper{
		ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
			return nil, projectScopedDefaultProjectErr()
		},
		ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
			return &resolvedProject{UUID: "project-uuid"}, nil
		},
		ListResourceGroupsFunc: func(context.Context, ...converged.ODataOption) ([]multidomainv4.ResourceGroup, error) {
			return []multidomainv4.ResourceGroup{{ExtId: new("rg-uuid")}}, nil
		},
		ListPrismElementsByResourceGroupFunc: func(_ context.Context, rgUUID string) ([]converged.PrismElementInfo, error) {
			assert.Equal(t, "rg-uuid", rgUUID)
			return []converged.PrismElementInfo{{ExtId: "pe-uuid", Name: "pe-name"}}, nil
		},
		ListClusterPhysicalGPUsFunc: func(_ context.Context, peUUID string) ([]clustermgmtv4.PhysicalGpuProfile, error) {
			assert.Equal(t, "pe-uuid", peUUID)
			return []clustermgmtv4.PhysicalGpuProfile{{
				PhysicalGpuConfig: &clustermgmtv4.PhysicalGpuConfig{DeviceName: new("A100")},
			}}, nil
		},
		ListClusterVirtualGPUsFunc: func(context.Context, string) ([]clustermgmtv4.VirtualGpuProfile, error) {
			return nil, nil
		},
	}
	check := &gpuCheck{
		machineDetails: &carenv1.NutanixMachineDetails{
			Cluster: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: new("pe-name"),
			},
			Project: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("project-uuid"),
			},
			GPUs: []capxv1.NutanixGPU{{
				Type: capxv1.NutanixGPUIdentifierName,
				Name: new("A100"),
			}},
		},
		field:     "test-field",
		nclient:   client,
		pcVersion: "7.6",
		projectRef: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierUUID,
			UUID: new("project-uuid"),
		},
	}

	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
	assert.Empty(t, result.Causes)
}

// vgpuProfile builds an AHV virtual GPU profile with a per-VM instance limit.
func vgpuProfile(name, extID string, maxPerVM int64) clustermgmtv4ahv.VirtualGpuProfile {
	return clustermgmtv4ahv.VirtualGpuProfile{
		ExtId: new(extID),
		Configuration: &clustermgmtv4ahv.VirtualGpuProfileConfiguration{
			Name:              new(name),
			MaxInstancesPerVm: new(maxPerVM),
		},
	}
}

// profileGPU builds a profile-typed GPU entry referencing a profile by name.
func profileGPUByName(name string) capxv1.NutanixGPU {
	return capxv1.NutanixGPU{
		Type: capxv1.NutanixGPUIdentifierProfile,
		Profile: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: new(name),
		},
	}
}

// profileGPUByUUID builds a profile-typed GPU entry referencing a profile by UUID.
func profileGPUByUUID(uuid string) capxv1.NutanixGPU {
	return capxv1.NutanixGPU{
		Type: capxv1.NutanixGPUIdentifierProfile,
		Profile: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierUUID,
			UUID: new(uuid),
		},
	}
}

func deviceGPUByName(name string) capxv1.NutanixGPU {
	return capxv1.NutanixGPU{
		Type: capxv1.NutanixGPUIdentifierName,
		Name: new(name),
	}
}

func deviceGPUByID(id int64) capxv1.NutanixGPU {
	return capxv1.NutanixGPU{
		Type:     capxv1.NutanixGPUIdentifierDeviceID,
		DeviceID: new(id),
	}
}

// repeatGPU returns n copies of gpu, for building gpus[] lists that exercise a per-VM instance limit.
func repeatGPU(gpu capxv1.NutanixGPU, n int) []capxv1.NutanixGPU {
	gpus := make([]capxv1.NutanixGPU, n)
	for i := range gpus {
		gpus[i] = gpu
	}
	return gpus
}

// TestGPUCheckProfileComposition covers the two per-machine GPU profile rules: every profile-typed
// entry must resolve to the same profile (physical or virtual, by name or UUID), and a profile may be
// requested at most a certain number of times per machine — its own maxInstancesPerVm for a virtual
// profile, or the fixed maxPhysicalGPUInstancesPerVM platform limit for a physical profile.
func TestGPUCheckProfileComposition(t *testing.T) {
	t.Parallel()

	// "A40-12C" allows 16 per VM; "A40-1B" allows only 1, mirroring real AHV profile limits.
	profiles := []clustermgmtv4ahv.VirtualGpuProfile{
		vgpuProfile("NVIDIA A40-12C", "uuid-12c", 16),
		vgpuProfile("NVIDIA A40-1B", "uuid-1b", 1),
		vgpuProfile("NVIDIA A40-2B", "uuid-2b", 2),
	}
	// Physical GPU profiles carry no maxInstancesPerVm, so only the uniqueness rule applies to them.
	physicalProfiles := []clustermgmtv4ahv.PhysicalGpuProfile{
		{
			ExtId:         new("uuid-phys-a40"),
			Configuration: &clustermgmtv4ahv.PhysicalGpuProfileConfiguration{Name: new("NVIDIA A40")},
		},
		{
			ExtId:         new("uuid-phys-a16"),
			Configuration: &clustermgmtv4ahv.PhysicalGpuProfileConfiguration{Name: new("NVIDIA A16")},
		},
	}

	tests := []struct {
		name        string
		gpus        []capxv1.NutanixGPU
		wantOK      bool
		wantField   string
		wantMessage string
	}{
		{
			name:   "same profile twice, within maxInstancesPerVm",
			gpus:   []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40-12C"), profileGPUByName("NVIDIA A40-12C")},
			wantOK: true,
		},
		{
			name:   "same profile by name and by UUID is one profile, not two",
			gpus:   []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40-12C"), profileGPUByUUID("uuid-12c")},
			wantOK: true,
		},
		{
			name:      "two different profiles on one machine",
			gpus:      []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40-12C"), profileGPUByName("NVIDIA A40-1B")},
			wantOK:    false,
			wantField: "test-field.gpus[1]",
			wantMessage: `GPU Profiles with different types cannot be added to the same machine, but ` +
				`"NVIDIA A40-12C" and "NVIDIA A40-1B" are both requested.`,
		},
		{
			name:      "different profiles referenced by name and UUID",
			gpus:      []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40-12C"), profileGPUByUUID("uuid-1b")},
			wantOK:    false,
			wantField: "test-field.gpus[1]",
			wantMessage: `GPU Profiles with different types cannot be added to the same machine, but ` +
				`"NVIDIA A40-12C" and "NVIDIA A40-1B" are both requested.`,
		},
		{
			name:        "same profile exceeds maxInstancesPerVm",
			gpus:        []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40-1B"), profileGPUByName("NVIDIA A40-1B")},
			wantOK:      false,
			wantField:   "test-field.gpus[1]",
			wantMessage: `GPU Profile "NVIDIA A40-1B" is requested 2 times on a single machine, but it allows at most 1 instances per machine.`, //nolint:lll // Message is long.
		},
		{
			name:   "single instance of a profile limited to one per VM",
			gpus:   []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40-1B")},
			wantOK: true,
		},
		{
			// The two rules treat this case differently, deliberately: for uniqueness the entries are
			// one profile (same extId), but for counting they are two attachments, because each gpus[]
			// entry is a separate GPU on the VM regardless of how it names the profile.
			name:      "same profile by name and by UUID still counts as two instances",
			gpus:      []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40-1B"), profileGPUByUUID("uuid-1b")},
			wantOK:    false,
			wantField: "test-field.gpus[1]",
			wantMessage: `GPU Profile "NVIDIA A40-1B" is requested 2 times on a single machine, ` +
				`but it allows at most 1 instances per machine.`,
		},
		{
			// Physical profiles report no maxInstancesPerVm, so uniqueness plus the fixed platform
			// limit (maxPhysicalGPUInstancesPerVM) constrain them instead.
			name:   "same physical profile twice is allowed",
			gpus:   []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40"), profileGPUByName("NVIDIA A40")},
			wantOK: true,
		},
		{
			name:   "physical profile requested exactly at the platform limit",
			gpus:   repeatGPU(profileGPUByName("NVIDIA A40"), maxPhysicalGPUInstancesPerVM),
			wantOK: true,
		},
		{
			name:      "physical profile exceeds the platform limit",
			gpus:      repeatGPU(profileGPUByName("NVIDIA A40"), maxPhysicalGPUInstancesPerVM+1),
			wantOK:    false,
			wantField: fmt.Sprintf("test-field.gpus[%d]", maxPhysicalGPUInstancesPerVM),
			wantMessage: fmt.Sprintf(
				`GPU Profile "NVIDIA A40" is requested %d times on a single machine, but it allows at most %d instances per machine.`, //nolint:lll // Message is long.
				maxPhysicalGPUInstancesPerVM+1,
				maxPhysicalGPUInstancesPerVM,
			),
		},
		{
			name:      "two different physical profiles on one machine",
			gpus:      []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40"), profileGPUByName("NVIDIA A16")},
			wantOK:    false,
			wantField: "test-field.gpus[1]",
			wantMessage: `GPU Profiles with different types cannot be added to the same machine, but ` +
				`"NVIDIA A40" and "NVIDIA A16" are both requested.`,
		},
		{
			name:      "a physical and a virtual profile on one machine",
			gpus:      []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40"), profileGPUByName("NVIDIA A40-12C")},
			wantOK:    false,
			wantField: "test-field.gpus[1]",
			wantMessage: `GPU Profiles with different types cannot be added to the same machine, but ` +
				`"NVIDIA A40" and "NVIDIA A40-12C" are both requested.`,
		},
		{
			name: "same profile exceeds a limit greater than one",
			gpus: []capxv1.NutanixGPU{
				profileGPUByName("NVIDIA A40-2B"), profileGPUByName("NVIDIA A40-2B"),
				profileGPUByName("NVIDIA A40-2B"),
			},
			wantOK:      false,
			wantField:   "test-field.gpus[2]",
			wantMessage: `GPU Profile "NVIDIA A40-2B" is requested 3 times on a single machine, but it allows at most 2 instances per machine.`, //nolint:lll // Message is long.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			check := &gpuCheck{
				machineDetails: &carenv1.NutanixMachineDetails{
					Cluster: &capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierUUID,
						UUID: new("pe-uuid"),
					},
					GPUs: tt.gpus,
				},
				field:     "test-field",
				pcVersion: "7.6",
				nclient: &clientWrapper{
					ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
						return &PCDefaultProject{UUID: "default-project"}, nil
					},
					ListAHVPhysicalGPUProfilesFunc: func(
						context.Context, string,
					) ([]clustermgmtv4ahv.PhysicalGpuProfile, error) {
						return physicalProfiles, nil
					},
					ListAHVVirtualGPUProfilesFunc: func(
						context.Context, string,
					) ([]clustermgmtv4ahv.VirtualGpuProfile, error) {
						return profiles, nil
					},
				},
			}

			result := check.Run(context.Background())
			assert.Equal(t, tt.wantOK, result.Allowed)
			assert.False(t, result.InternalError)
			if tt.wantOK {
				assert.Empty(t, result.Causes)
				return
			}
			require.Len(t, result.Causes, 1)
			assert.Equal(t, tt.wantMessage, result.Causes[0].Message)
			assert.Equal(t, tt.wantField, result.Causes[0].Field)
		})
	}
}

func TestGPUCheckMixedDeviceAndProfile(t *testing.T) {
	t.Parallel()

	const mixMessage = "GPU devices identified by name or device ID cannot be mixed with GPU profiles on the same machine."

	tests := []struct {
		name        string
		gpus        []capxv1.NutanixGPU
		wantOK      bool
		wantField   string
		wantMessage string
	}{
		{
			name:   "name and device ID together are allowed",
			gpus:   []capxv1.NutanixGPU{deviceGPUByName("A100"), deviceGPUByID(42)},
			wantOK: true,
		},
		{
			name:        "name cannot be mixed with profile",
			gpus:        []capxv1.NutanixGPU{deviceGPUByName("A100"), profileGPUByName("NVIDIA A40-12C")},
			wantOK:      false,
			wantField:   "test-field.gpus[1]",
			wantMessage: mixMessage,
		},
		{
			name:        "device ID cannot be mixed with profile",
			gpus:        []capxv1.NutanixGPU{deviceGPUByID(42), profileGPUByName("NVIDIA A40-12C")},
			wantOK:      false,
			wantField:   "test-field.gpus[1]",
			wantMessage: mixMessage,
		},
		{
			name:        "profile cannot be mixed with name",
			gpus:        []capxv1.NutanixGPU{profileGPUByName("NVIDIA A40-12C"), deviceGPUByName("A100")},
			wantOK:      false,
			wantField:   "test-field.gpus[1]",
			wantMessage: mixMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			check := &gpuCheck{
				machineDetails: &carenv1.NutanixMachineDetails{
					Cluster: &capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierUUID,
						UUID: new("pe-uuid"),
					},
					GPUs: tt.gpus,
				},
				field:     "test-field",
				pcVersion: "7.6",
				nclient: &clientWrapper{
					ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
						return &PCDefaultProject{UUID: "default-project"}, nil
					},
					ListClusterPhysicalGPUsFunc: func(context.Context, string) ([]clustermgmtv4.PhysicalGpuProfile, error) {
						return []clustermgmtv4.PhysicalGpuProfile{{
							PhysicalGpuConfig: &clustermgmtv4.PhysicalGpuConfig{
								DeviceName: new("A100"),
								DeviceId:   new(int64(42)),
							},
						}}, nil
					},
					ListClusterVirtualGPUsFunc: func(context.Context, string) ([]clustermgmtv4.VirtualGpuProfile, error) {
						return nil, nil
					},
					ListAHVPhysicalGPUProfilesFunc: func(
						context.Context, string,
					) ([]clustermgmtv4ahv.PhysicalGpuProfile, error) {
						return nil, nil
					},
					ListAHVVirtualGPUProfilesFunc: func(
						context.Context, string,
					) ([]clustermgmtv4ahv.VirtualGpuProfile, error) {
						return []clustermgmtv4ahv.VirtualGpuProfile{
							vgpuProfile("NVIDIA A40-12C", "uuid-12c", 16),
						}, nil
					},
				},
			}

			result := check.Run(context.Background())
			assert.Equal(t, tt.wantOK, result.Allowed)
			assert.False(t, result.InternalError)
			if tt.wantOK {
				assert.Empty(t, result.Causes)
				return
			}
			require.Len(t, result.Causes, 1)
			assert.Equal(t, tt.wantMessage, result.Causes[0].Message)
			assert.Equal(t, tt.wantField, result.Causes[0].Field)
		})
	}
}

// TestGPUCheckMixedDeviceAndProfileReportsEveryMismatch verifies validateGPUDeviceAndProfileMix collects a
// Cause for every mismatched entry instead of stopping at the first, so a machine with multiple bad gpus[]
// entries is fully diagnosed in one admission response.
func TestGPUCheckMixedDeviceAndProfileReportsEveryMismatch(t *testing.T) {
	t.Parallel()

	const mixMessage = "GPU devices identified by name or device ID cannot be mixed with GPU profiles on the same machine."

	check := &gpuCheck{
		machineDetails: &carenv1.NutanixMachineDetails{
			Cluster: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("pe-uuid"),
			},
			GPUs: []capxv1.NutanixGPU{
				deviceGPUByName("A100"),
				profileGPUByName("NVIDIA A40-12C"),
				profileGPUByName("NVIDIA A40-24C"),
			},
		},
		field:     "test-field",
		pcVersion: "7.6",
	}

	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	require.Len(t, result.Causes, 2)
	assert.Equal(t, mixMessage, result.Causes[0].Message)
	assert.Equal(t, "test-field.gpus[1]", result.Causes[0].Field)
	assert.Equal(t, mixMessage, result.Causes[1].Message)
	assert.Equal(t, "test-field.gpus[2]", result.Causes[1].Field)
}

func TestGPUCheckResolvesPEFromFailureDomain(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, capxv1.AddToScheme(scheme))
	kclient := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(&capxv1.NutanixFailureDomain{
		ObjectMeta: metav1.ObjectMeta{Name: "fd-1", Namespace: "default"},
		Spec: capxv1.NutanixFailureDomainSpec{
			PrismElementCluster: capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("pe-uuid"),
			},
		},
	}).Build()

	check := &gpuCheck{
		machineDetails: &carenv1.NutanixMachineDetails{
			GPUs: []capxv1.NutanixGPU{{
				Type: capxv1.NutanixGPUIdentifierName,
				Name: new("A100"),
			}},
		},
		field:             "test-field",
		failureDomainName: "fd-1",
		namespace:         "default",
		kclient:           kclient,
		pcVersion:         "7.5",
		nclient: &clientWrapper{
			ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
				return &PCDefaultProject{UUID: "default-project"}, nil
			},
			ListClusterPhysicalGPUsFunc: func(_ context.Context, peUUID string) ([]clustermgmtv4.PhysicalGpuProfile, error) {
				assert.Equal(t, "pe-uuid", peUUID)
				return []clustermgmtv4.PhysicalGpuProfile{{
					PhysicalGpuConfig: &clustermgmtv4.PhysicalGpuConfig{DeviceName: new("A100")},
				}}, nil
			},
			ListClusterVirtualGPUsFunc: func(context.Context, string) ([]clustermgmtv4.VirtualGpuProfile, error) {
				return nil, nil
			},
		},
	}

	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
	assert.Empty(t, result.Causes)
}
