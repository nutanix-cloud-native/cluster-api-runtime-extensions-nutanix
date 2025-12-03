// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestInitFailureDomainChecks(t *testing.T) {
	testCases := []struct {
		name                     string
		nutanixClusterConfigSpec *carenv1.NutanixClusterConfigSpec
		machineDeployments       []clusterv1.MachineDeploymentTopology
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
			machineDeployments:  []clusterv1.MachineDeploymentTopology{},
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
			machineDeployments:  []clusterv1.MachineDeploymentTopology{},
			nclient:             &clientWrapper{},
			kclient:             getK8sClient(),
			expectedChecksCount: 3,
		},
		{
			name:                     "worker machines with failureDomains",
			nutanixClusterConfigSpec: nil,
			machineDeployments: []clusterv1.MachineDeploymentTopology{{
				Class:         "default-worker",
				Name:          "md-1",
				FailureDomain: ptr.To("fd-w1"),
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
			machineDeployments: []clusterv1.MachineDeploymentTopology{{
				Class:         "default-worker",
				Name:          "md-1",
				FailureDomain: ptr.To("fd-w1"),
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
				cluster: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Topology: &clusterv1.Topology{
							Workers: &clusterv1.WorkersTopology{
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
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					return nil, nil
				},
				ListClustersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
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
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To(peClusterName),
							ExtId: ptr.To(peClusterUUID),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				GetSubnetByIdFunc: func(uuid *string, args ...map[string]interface{}) (*netv4.GetSubnetApiResponse, error) {
					return nil, nil
				},
				ListSubnetsFunc: func(
					page_ *int,
					limit_ *int,
					filter_ *string,
					orderby_ *string,
					expand_ *string,
					select_ *string,
					args ...map[string]interface{},
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
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To(peClusterName),
							ExtId: ptr.To(peClusterUUID),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				GetSubnetByIdFunc: func(uuid *string, args ...map[string]interface{}) (*netv4.GetSubnetApiResponse, error) {
					return nil, nil
				},
				ListSubnetsFunc: func(
					page_ *int,
					limit_ *int,
					filter_ *string,
					orderby_ *string,
					expand_ *string,
					select_ *string,
					args ...map[string]interface{},
				) (*netv4.ListSubnetsApiResponse, error) {
					resp := &netv4.ListSubnetsApiResponse{
						ObjectType_: ptr.To("networking.v4.config.ListSubnetsApiResponse"),
					}
					err := resp.SetData([]netv4.Subnet{
						{
							Name:  ptr.To(subnetName),
							ExtId: ptr.To(subnetUUID),
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

func getK8sClient() ctrlclient.Client {
	fdObj := &capxv1.NutanixFailureDomain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      failureDomainName,
			Namespace: namespace,
		},
		Spec: capxv1.NutanixFailureDomainSpec{
			PrismElementCluster: capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To(string(peClusterName)),
			},
			Subnets: []capxv1.NutanixResourceIdentifier{{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To(subnetName),
			}},
		},
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(capxv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(fdObj).Build()
}
