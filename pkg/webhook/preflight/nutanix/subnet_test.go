// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	multidomainv4 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	converged "github.com/nutanix-cloud-native/prism-go-client/converged"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const (
	subnetTestProjectUUID = "00000000-0000-0000-0000-0000000000aa"
	subnetTestField       = "test.field.subnets[0]"
)

func subnetNameID(name string) capxv1.NutanixResourceIdentifier {
	return capxv1.NutanixResourceIdentifier{Type: capxv1.NutanixIdentifierName, Name: new(name)}
}

func subnetUUIDID(uuid string) capxv1.NutanixResourceIdentifier {
	return capxv1.NutanixResourceIdentifier{Type: capxv1.NutanixIdentifierUUID, UUID: new(uuid)}
}

func TestSubnetCheck(t *testing.T) {
	t.Parallel()

	// projectScopedClient builds a client that reports a project-scoped user resolving to
	// subnetTestProjectUUID, with the given subnets and resource-group Prism Elements.
	testCases := []struct {
		name                  string
		subnetID              capxv1.NutanixResourceIdentifier
		clusterRef            *capxv1.NutanixResourceIdentifier
		projectRef            *capxv1.NutanixResourceIdentifier
		pcVersion             string
		nclient               client
		expectedAllowed       bool
		expectedInternalError bool
		expectedCauseMessage  string
	}{
		{
			name:       "pre-7.6 subnet by name on referenced PE - pass",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetNameID(peClusterName)),
			pcVersion:  "7.3",
			nclient: &clientWrapper{
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{
						{Name: new(subnetName), ExtId: new(subnetUUID), ClusterReference: new(peClusterUUID)},
					}, nil
				},
				ListClustersFunc: func(
					_ context.Context, _, _ *int, _, _, _, _ *string, _ ...map[string]any,
				) (*clustermgmtv4.ListClustersApiResponse, error) {
					resp := &clustermgmtv4.ListClustersApiResponse{}
					require.NoError(t, resp.SetData([]clustermgmtv4.Cluster{
						{Name: new(peClusterName), ExtId: new(peClusterUUID)},
					}))
					return resp, nil
				},
			},
			expectedAllowed: true,
		},
		{
			name:       "cluster by uuid short-circuits cluster lookup - pass",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetUUIDID(peClusterUUID)),
			pcVersion:  "7.3",
			nclient: &clientWrapper{
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{
						{Name: new(subnetName), ExtId: new(subnetUUID), ClusterReference: new(peClusterUUID)},
					}, nil
				},
				// No cluster funcs are set: if getClusters were called this case would fail.
			},
			expectedAllowed: true,
		},
		{
			name:       "overlay subnet by SubnetType - pass",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetUUIDID(peClusterUUID)),
			pcVersion:  "7.3",
			nclient: &clientWrapper{
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{
						{
							Name:                 new(subnetName),
							ExtId:                new(subnetUUID),
							SubnetType:           netv4.SUBNETTYPE_OVERLAY.Ref(),
							ClusterReference:     nil,
							ClusterReferenceList: nil,
						},
					}, nil
				},
			},
			expectedAllowed: true,
		},
		{
			name:       "PC vSwitch subnet matched via ClusterReferenceList only - pass",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetUUIDID(peClusterUUID)),
			pcVersion:  "7.3",
			nclient: &clientWrapper{
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{
						{
							Name:                 new(subnetName),
							ExtId:                new(subnetUUID),
							SubnetType:           netv4.SUBNETTYPE_VLAN.Ref(),
							ClusterReference:     nil,
							ClusterReferenceList: []string{"other-pe-uuid", peClusterUUID},
						},
					}, nil
				},
			},
			expectedAllowed: true,
		},
		{
			name:       "nil ClusterReference without overlay type is not a match - fail",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetUUIDID(peClusterUUID)),
			pcVersion:  "7.3",
			nclient: &clientWrapper{
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{
						{
							Name:             new(subnetName),
							ExtId:            new(subnetUUID),
							ClusterReference: nil,
						},
					}, nil
				},
			},
			expectedAllowed:      false,
			expectedCauseMessage: "Found 0 Subnets",
		},
		{
			name:       "subnet not found - fail",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetUUIDID(peClusterUUID)),
			pcVersion:  "7.3",
			nclient: &clientWrapper{
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{}, nil
				},
			},
			expectedAllowed:      false,
			expectedCauseMessage: "Found 0 Subnets",
		},
		{
			name:       "subnet attached to a different PE - fail",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetUUIDID(peClusterUUID)),
			pcVersion:  "7.3",
			nclient: &clientWrapper{
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{
						{
							Name:             new(subnetName),
							ExtId:            new(subnetUUID),
							ClusterReference: new("other-pe-uuid"),
						},
					}, nil
				},
			},
			expectedAllowed:      false,
			expectedCauseMessage: "Found 0 Subnets",
		},
		{
			name:       "ClusterReferenceList does not contain PE - fail",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetUUIDID(peClusterUUID)),
			pcVersion:  "7.3",
			nclient: &clientWrapper{
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{
						{
							Name:                 new(subnetName),
							ExtId:                new(subnetUUID),
							SubnetType:           netv4.SUBNETTYPE_VLAN.Ref(),
							ClusterReferenceList: []string{"other-pe-uuid"},
						},
					}, nil
				},
			},
			expectedAllowed:      false,
			expectedCauseMessage: "Found 0 Subnets",
		},
		{
			name:       "project-scoped cluster by name resolved via resource groups - pass",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetNameID(peClusterName)),
			projectRef: new(subnetUUIDID(subnetTestProjectUUID)),
			pcVersion:  "7.6",
			nclient: &clientWrapper{
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					return nil, projectScopedDefaultProjectErr()
				},
				ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
					return &resolvedProject{UUID: subnetTestProjectUUID}, nil
				},
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{
						{Name: new(subnetName), ExtId: new(subnetUUID), ClusterReference: new(peClusterUUID)},
					}, nil
				},
				ListResourceGroupsFunc: func(
					context.Context, ...converged.ODataOption,
				) ([]multidomainv4.ResourceGroup, error) {
					return []multidomainv4.ResourceGroup{{ExtId: new("rg-1"), Name: new("rg-1")}}, nil
				},
				ListPrismElementsByResourceGroupFunc: func(
					_ context.Context, _ string,
				) ([]converged.PrismElementInfo, error) {
					return []converged.PrismElementInfo{{ExtId: peClusterUUID, Name: peClusterName}}, nil
				},
				// No cluster funcs: project-scoped path must not call getClusters.
			},
			expectedAllowed: true,
		},
		{
			name:       "project-scoped duplicate on same PE prefers owned over shared - pass",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetUUIDID(peClusterUUID)),
			projectRef: new(subnetUUIDID(subnetTestProjectUUID)),
			pcVersion:  "7.6",
			nclient: &clientWrapper{
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					return nil, projectScopedDefaultProjectErr()
				},
				ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
					return &resolvedProject{UUID: subnetTestProjectUUID}, nil
				},
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{
						{
							Name:             new(subnetName),
							ExtId:            new("owned-subnet-uuid"),
							ClusterReference: new(peClusterUUID),
							ProjectExtId:     new(subnetTestProjectUUID),
						},
						{
							Name:               new(subnetName),
							ExtId:              new("shared-subnet-uuid"),
							ClusterReference:   new(peClusterUUID),
							SharedWithProjects: []string{subnetTestProjectUUID},
						},
					}, nil
				},
			},
			expectedAllowed: true,
		},
		{
			name:       "project-scoped PE not available in project scope - fail",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetNameID(peClusterName)),
			projectRef: new(subnetUUIDID(subnetTestProjectUUID)),
			pcVersion:  "7.6",
			nclient: &clientWrapper{
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					return nil, projectScopedDefaultProjectErr()
				},
				ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
					return &resolvedProject{UUID: subnetTestProjectUUID}, nil
				},
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return []netv4.Subnet{
						{Name: new(subnetName), ExtId: new(subnetUUID), ClusterReference: new(peClusterUUID)},
					}, nil
				},
				ListResourceGroupsFunc: func(
					context.Context, ...converged.ODataOption,
				) ([]multidomainv4.ResourceGroup, error) {
					return []multidomainv4.ResourceGroup{{ExtId: new("rg-1"), Name: new("rg-1")}}, nil
				},
				ListPrismElementsByResourceGroupFunc: func(
					_ context.Context, _ string,
				) ([]converged.PrismElementInfo, error) {
					return []converged.PrismElementInfo{{ExtId: "some-other-uuid", Name: "some-other-pe"}}, nil
				},
			},
			expectedAllowed:      false,
			expectedCauseMessage: "not available in the selected project scope",
		},
		{
			name:       "project scope resolution error - internal error",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetNameID(peClusterName)),
			projectRef: new(subnetUUIDID(subnetTestProjectUUID)),
			pcVersion:  "7.6",
			nclient: &clientWrapper{
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					return nil, fmt.Errorf("unexpected server failure")
				},
			},
			expectedAllowed:       false,
			expectedInternalError: true,
			expectedCauseMessage:  "Failed to resolve project scope for subnet validation",
		},
		{
			name:       "subnet listing error - internal error",
			subnetID:   subnetNameID(subnetName),
			clusterRef: new(subnetUUIDID(peClusterUUID)),
			pcVersion:  "7.3",
			nclient: &clientWrapper{
				ListSubnetsByFilterFunc: func(_ context.Context, _ string) ([]netv4.Subnet, error) {
					return nil, fmt.Errorf("boom")
				},
			},
			expectedAllowed:       false,
			expectedInternalError: true,
			expectedCauseMessage:  "Failed to get subnet",
		},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			check := &subnetCheck{
				subnetID:   tc.subnetID,
				clusterRef: tc.clusterRef,
				field:      subnetTestField,
				nclient:    tc.nclient,
				pcVersion:  tc.pcVersion,
				projectRef: tc.projectRef,
			}

			result := check.Run(ctx)

			assert.Equal(t, tc.expectedAllowed, result.Allowed)
			assert.Equal(t, tc.expectedInternalError, result.InternalError)
			if tc.expectedCauseMessage != "" {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, tc.expectedCauseMessage)
			} else {
				assert.Empty(t, result.Causes)
			}
		})
	}
}

func TestNewSubnetChecks(t *testing.T) {
	t.Parallel()

	cpMachineDetails := func(cluster *capxv1.NutanixResourceIdentifier, subnetCount int) carenv1.NutanixMachineDetails {
		subnets := make([]capxv1.NutanixResourceIdentifier, 0, subnetCount)
		for i := range subnetCount {
			subnets = append(subnets, subnetNameID(fmt.Sprintf("subnet-%d", i)))
		}
		return carenv1.NutanixMachineDetails{Cluster: cluster, Subnets: subnets}
	}

	peName := new(subnetNameID(peClusterName))

	testCases := []struct {
		name          string
		cd            *checkDependencies
		expectedCount int
	}{
		{name: "nil dependencies", cd: nil, expectedCount: 0},
		{name: "nil client", cd: &checkDependencies{pcVersion: "7.3"}, expectedCount: 0},
		{
			name:          "empty pc version",
			cd:            &checkDependencies{nclient: &clientWrapper{}},
			expectedCount: 0,
		},
		{
			name: "control plane with cluster and two subnets",
			cd: &checkDependencies{
				nclient:   &clientWrapper{},
				pcVersion: "7.3",
				nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
					ControlPlane: &carenv1.NutanixControlPlaneSpec{
						Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
							MachineDetails: cpMachineDetails(peName, 2),
						},
					},
				},
			},
			expectedCount: 2,
		},
		{
			name: "control plane without cluster is skipped",
			cd: &checkDependencies{
				nclient:   &clientWrapper{},
				pcVersion: "7.3",
				nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
					ControlPlane: &carenv1.NutanixControlPlaneSpec{
						Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
							MachineDetails: cpMachineDetails(nil, 2),
						},
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "control plane and worker",
			cd: &checkDependencies{
				nclient:   &clientWrapper{},
				pcVersion: "7.3",
				nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
					ControlPlane: &carenv1.NutanixControlPlaneSpec{
						Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
							MachineDetails: cpMachineDetails(peName, 1),
						},
					},
				},
				nutanixWorkerNodeConfigSpecByMachineDeploymentName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
					"md-1": {
						Nutanix: &carenv1.NutanixWorkerNodeSpec{
							MachineDetails: cpMachineDetails(peName, 2),
						},
					},
				},
			},
			expectedCount: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checks := newSubnetChecks(tc.cd)
			assert.Len(t, checks, tc.expectedCount)
			for _, c := range checks {
				assert.Equal(t, "NutanixSubnet", c.Name())
			}
		})
	}
}

func TestSubnetBelongsToPrismElement(t *testing.T) {
	t.Parallel()

	otherUUID := "22222222-2222-2222-2222-222222222222"
	anotherUUID := "33333333-3333-3333-3333-333333333333"

	tests := []struct {
		name     string
		subnet   *netv4.Subnet
		expected bool
	}{
		{
			name:     "overlay subnet",
			subnet:   &netv4.Subnet{SubnetType: netv4.SUBNETTYPE_OVERLAY.Ref()},
			expected: true,
		},
		{
			name:     "matching ClusterReference",
			subnet:   &netv4.Subnet{ClusterReference: new(peClusterUUID)},
			expected: true,
		},
		{
			name:     "non-matching ClusterReference",
			subnet:   &netv4.Subnet{ClusterReference: new(otherUUID)},
			expected: false,
		},
		{
			name:     "matching UUID in ClusterReferenceList",
			subnet:   &netv4.Subnet{ClusterReferenceList: []string{otherUUID, peClusterUUID, anotherUUID}},
			expected: true,
		},
		{
			name:     "non-matching ClusterReferenceList",
			subnet:   &netv4.Subnet{ClusterReferenceList: []string{otherUUID, anotherUUID}},
			expected: false,
		},
		{
			name: "ClusterReference matches even if ClusterReferenceList does not",
			subnet: &netv4.Subnet{
				ClusterReference:     new(peClusterUUID),
				ClusterReferenceList: []string{otherUUID},
			},
			expected: true,
		},
		{
			name: "ClusterReferenceList matches even if ClusterReference does not",
			subnet: &netv4.Subnet{
				ClusterReference:     new(otherUUID),
				ClusterReferenceList: []string{peClusterUUID},
			},
			expected: true,
		},
		{
			name:     "nil ClusterReference and nil ClusterReferenceList",
			subnet:   &netv4.Subnet{},
			expected: false,
		},
		{
			name:     "nil ClusterReference and empty ClusterReferenceList",
			subnet:   &netv4.Subnet{ClusterReferenceList: []string{}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, subnetBelongsToPrismElement(tt.subnet, peClusterUUID))
		})
	}
}
