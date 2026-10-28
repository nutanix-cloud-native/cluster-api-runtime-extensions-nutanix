// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	networkingcommonapi "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/common/v1/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestInitFailureDomainChecks(t *testing.T) {
	testCases := []struct {
		name                     string
		nutanixClusterConfigSpec *carenv1.NutanixClusterConfigSpec
		machineDeployments       []clusterv1beta2.MachineDeploymentTopology
		nclient                  client
		kclient                  ctrlclient.Client
		expectedChecksCount      int
	}{
		{
			name:                     "client not initialized",
			nutanixClusterConfigSpec: nil,
			machineDeployments:       nil,
			nclient:                  nil,
			kclient:                  nil,
			expectedChecksCount:      0,
		},
		{
			name:                     "nil cluster config",
			nutanixClusterConfigSpec: nil,
			machineDeployments:       nil,
			nclient:                  &clientWrapper{},
			kclient:                  getK8sClient(),
			expectedChecksCount:      0,
		},
		{
			name: "cluster config without controlPlane failureDomains",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{},
				},
			},
			machineDeployments:  []clusterv1beta2.MachineDeploymentTopology{},
			nclient:             &clientWrapper{},
			kclient:             getK8sClient(),
			expectedChecksCount: 0,
		},
		{
			name: "cluster config with controlPlane failureDomains",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{"fd-1", "fd-2", "fd-3"},
					},
				},
			},
			machineDeployments:  []clusterv1beta2.MachineDeploymentTopology{},
			nclient:             &clientWrapper{},
			kclient:             getK8sClient(),
			expectedChecksCount: 3,
		},
		{
			name:                     "worker machines with failureDomains",
			nutanixClusterConfigSpec: nil,
			machineDeployments: []clusterv1beta2.MachineDeploymentTopology{{
				Class:         "default-worker",
				Name:          "md-1",
				FailureDomain: "fd-w1",
			}},
			nclient:             &clientWrapper{},
			kclient:             getK8sClient(),
			expectedChecksCount: 1,
		},
		{
			name: "cluster config with controlPlane failureDomains and worker machines with failureDomains",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{"fd-1", "fd-2", "fd-3"},
					},
				},
			},
			machineDeployments: []clusterv1beta2.MachineDeploymentTopology{{
				Class:         "default-worker",
				Name:          "md-1",
				FailureDomain: "fd-w1",
			}},
			nclient:             &clientWrapper{},
			kclient:             getK8sClient(),
			expectedChecksCount: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cd := &checkDependencies{
				nutanixClusterConfigSpec: tc.nutanixClusterConfigSpec,
				cluster: &clusterv1beta2.Cluster{
					Spec: clusterv1beta2.ClusterSpec{
						Topology: clusterv1beta2.Topology{
							Workers: clusterv1beta2.WorkersTopology{
								MachineDeployments: tc.machineDeployments,
							},
						},
					},
				},
				nclient:   tc.nclient,
				kclient:   tc.kclient,
				pcVersion: "7.3.0",
			}

			// Call the function under test
			checks := newFailureDomainChecks(cd)

			// Verify number of checks
			assert.Len(t, checks, tc.expectedChecksCount, "Wrong number of checks created")
		})
	}
}

const (
	failureDomainName = "fd-1"
	namespace         = "default"
	field             = "test.field.path"
	peClusterName     = "pe-1"
	peClusterUUID     = "pe-1-cluster-uuid"
	subnetName        = "pe-1-subnet1"
	subnetUUID        = "pe-1-subnet1-uuid"
)

func TestFailureDomainCheck(t *testing.T) {
	testCases := []struct {
		name                  string
		fdName                string
		kclient               ctrlclient.Client
		nclient               client
		expectedAllowed       bool
		expectedInternalError bool
		expectedCauseMessage  string
		field                 string
	}{
		{
			name:                  "failureDomain object not found",
			fdName:                failureDomainName,
			kclient:               fake.NewFakeClient(),
			nclient:               &clientWrapper{},
			expectedAllowed:       false,
			expectedInternalError: true,
			expectedCauseMessage:  "Failed to get NutanixFailureDomain",
			field:                 field,
		},
		{
			name:    "failureDomain check failed at PE cluster validation",
			fdName:  failureDomainName,
			kclient: getK8sClient(),
			nclient: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]any,
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					return nil, nil
				},
				ListClustersFunc: func(
					ctx context.Context,
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]any,
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					return nil, fmt.Errorf("failed to list the prism element clusters")
				},
			},
			expectedAllowed:       false,
			expectedInternalError: true,
			expectedCauseMessage:  "failed to list the prism element clusters",
			field:                 field,
		},
		{
			name:    "failureDomain check failed at subnets validation",
			fdName:  failureDomainName,
			kclient: getK8sClient(),
			nclient: &clientWrapper{
				ListClustersFunc: func(
					ctx context.Context,
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]any,
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: new("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  new(peClusterName),
							ExtId: new(peClusterUUID),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				GetSubnetByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*netv4.GetSubnetApiResponse, error) {
					return nil, nil
				},
				ListSubnetsFunc: func(
					ctx context.Context,
					page_ *int,
					limit_ *int,
					filter_ *string,
					orderby_ *string,
					expand_ *string,
					select_ *string,
					args ...map[string]any,
				) (*netv4.ListSubnetsApiResponse, error) {
					return nil, fmt.Errorf("failed to list subnets")
				},
			},
			expectedAllowed:       false,
			expectedInternalError: true,
			expectedCauseMessage:  "failed to list subnets",
			field:                 field,
		},
		{
			name:    "failureDomain check success",
			fdName:  failureDomainName,
			kclient: getK8sClient(),
			nclient: &clientWrapper{
				ListClustersFunc: func(
					ctx context.Context,
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]any,
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: new("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  new(peClusterName),
							ExtId: new(peClusterUUID),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				GetSubnetByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*netv4.GetSubnetApiResponse, error) {
					return nil, nil
				},
				ListSubnetsFunc: func(
					ctx context.Context,
					page_ *int,
					limit_ *int,
					filter_ *string,
					orderby_ *string,
					expand_ *string,
					select_ *string,
					args ...map[string]any,
				) (*netv4.ListSubnetsApiResponse, error) {
					resp := &netv4.ListSubnetsApiResponse{
						ObjectType_: new("networking.v4.config.ListSubnetsApiResponse"),
					}
					err := resp.SetData([]netv4.Subnet{
						{
							Name:             new(subnetName),
							ExtId:            new(subnetUUID),
							ClusterReference: new(peClusterUUID),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectedAllowed:       true,
			expectedInternalError: false,
			expectedCauseMessage:  "",
			field:                 field,
		},
	}

	ctx := context.TODO()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the check function
			check := failureDomainCheck{
				failureDomainName: failureDomainName,
				namespace:         namespace,
				kclient:           tc.kclient,
				nclient:           tc.nclient,
				field:             field,
			}

			// Run the check
			result := check.Run(ctx)

			// Verify the result
			assert.Equal(t, tc.expectedAllowed, result.Allowed)
			assert.Equal(t, tc.expectedInternalError, result.InternalError)

			if tc.expectedCauseMessage != "" {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, tc.expectedCauseMessage)
				assert.Equal(t, tc.field, result.Causes[0].Field)
			} else {
				assert.Empty(t, result.Causes)
			}
		})
	}
}

func TestFailureDomainCheckRejectsOverlappingPodCIDR(t *testing.T) {
	t.Parallel()

	check := failureDomainCheck{
		failureDomainName: failureDomainName,
		namespace:         namespace,
		field:             field,
		kclient:           getK8sClient(),
		podCIDRs:          []string{"10.22.0.0/16"},
		nclient: &clientWrapper{
			ListClustersFunc: func(
				context.Context,
				*int,
				*int,
				*string,
				*string,
				*string,
				*string,
				...map[string]any,
			) (*clustermgmtv4.ListClustersApiResponse, error) {
				resp := &clustermgmtv4.ListClustersApiResponse{}
				err := resp.SetData([]clustermgmtv4.Cluster{{
					Name:  new(peClusterName),
					ExtId: new(peClusterUUID),
				}})
				require.NoError(t, err)
				return resp, nil
			},
			ListSubnetsFunc: func(
				context.Context,
				*int,
				*int,
				*string,
				*string,
				*string,
				*string,
				...map[string]any,
			) (*netv4.ListSubnetsApiResponse, error) {
				resp := &netv4.ListSubnetsApiResponse{}
				err := resp.SetData([]netv4.Subnet{{
					Name:             new(subnetName),
					ClusterReference: new(peClusterUUID),
					IpConfig: []netv4.IPConfig{{
						Ipv4: &netv4.IPv4Config{
							IpSubnet: &netv4.IPv4Subnet{
								Ip:           &networkingcommonapi.IPv4Address{Value: new("10.22.1.1")},
								PrefixLength: new(24),
							},
						},
					}},
				}})
				require.NoError(t, err)
				return resp, nil
			},
		},
	}

	result := check.Run(context.Background())
	require.False(t, result.Allowed)
	require.Len(t, result.Causes, 1)
	assert.Contains(t, result.Causes[0].Message, `Pod CIDR "10.22.0.0/16" overlaps with node subnet`)
	assert.Equal(t, field, result.Causes[0].Field)
}

func getK8sClient() ctrlclient.Client {
	fdObj := &capxv1.NutanixFailureDomain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      failureDomainName,
			Namespace: namespace,
		},
		Spec: capxv1.NutanixFailureDomainSpec{
			PrismElementCluster: capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: new(string(peClusterName)),
			},
			Subnets: []capxv1.NutanixResourceIdentifier{{
				Type: capxv1.NutanixIdentifierName,
				Name: new(subnetName),
			}},
		},
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(capxv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(fdObj).Build()
}

func TestGetSubnetsInProject_UsesProjectFilter(t *testing.T) {
	t.Parallel()

	id := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: new("subnet-a"),
	}
	nclient := &clientWrapper{
		ListSubnetsFunc: func(
			ctx context.Context,
			page_ *int,
			limit_ *int,
			filter_ *string,
			orderby_ *string,
			expand_ *string,
			select_ *string,
			args ...map[string]any,
		) (*netv4.ListSubnetsApiResponse, error) {
			require.NotNil(t, filter_)
			assert.Equal(
				t,
				"name eq 'subnet-a' and (projectExtId eq 'project-uuid-1' or sharedWithProjects/any(a:a eq 'project-uuid-1'))",
				*filter_,
			)
			resp := &netv4.ListSubnetsApiResponse{}
			err := resp.SetData([]netv4.Subnet{{Name: new("subnet-a")}})
			require.NoError(t, err)
			return resp, nil
		},
	}

	subnets, err := getSubnetsInProject(context.Background(), nclient, id, "project-uuid-1")
	require.NoError(t, err)
	require.Len(t, subnets, 1)
}

func TestSubnetFromGetSubnetData(t *testing.T) {
	t.Parallel()

	valueSubnet := netv4.Subnet{
		ExtId: new("subnet-uuid"),
		Name:  new("subnet-name"),
	}

	tests := []struct {
		name        string
		data        any
		expectError bool
	}{
		{
			name:        "accepts value subnet",
			data:        valueSubnet,
			expectError: false,
		},
		{
			name:        "accepts pointer subnet",
			data:        &valueSubnet,
			expectError: false,
		},
		{
			name:        "rejects nil pointer subnet",
			data:        (*netv4.Subnet)(nil),
			expectError: true,
		},
		{
			name:        "rejects unexpected type",
			data:        "not-a-subnet",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			subnet, err := subnetFromGetSubnetData(tt.data)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, ptr.Deref(valueSubnet.ExtId, ""), ptr.Deref(subnet.ExtId, ""))
			assert.Equal(t, ptr.Deref(valueSubnet.Name, ""), ptr.Deref(subnet.Name, ""))
		})
	}
}
