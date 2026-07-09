// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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

const (
	metroName    = "metro-1"
	metroFD1     = "metro-fd-1"
	metroFD2     = "metro-fd-2"
	metroPE1UUID = "metro-pe-1-uuid"
	metroPE2UUID = "metro-pe-2-uuid"
	metroSubnet1 = "metro-subnet-1-uuid"
	metroSubnet2 = "metro-subnet-2-uuid"
	metroSubnet3 = "metro-subnet-3-uuid"
	metroSubnet4 = "metro-subnet-4-uuid"
)

// metroScheme returns a scheme with the CAPX types registered.
func metroScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(capxv1.AddToScheme(scheme))
	return scheme
}

// newMetroFailureDomain builds a NutanixFailureDomain referencing a PE and one
// or more subnets, all identified by UUID.
func newMetroFailureDomain(name, peUUID string, subnetUUIDs ...string) *capxv1.NutanixFailureDomain {
	subnets := make([]capxv1.NutanixResourceIdentifier, 0, len(subnetUUIDs))
	for _, subnetUUID := range subnetUUIDs {
		subnets = append(subnets, capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierUUID,
			UUID: ptr.To(subnetUUID),
		})
	}

	return &capxv1.NutanixFailureDomain{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: capxv1.NutanixFailureDomainSpec{
			PrismElementCluster: capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To(peUUID),
			},
			Subnets: subnets,
		},
	}
}

// newMetroObject builds a NutanixMetro referencing two failure domains.
func newMetroObject(name, fd1, fd2 string) *capxv1.NutanixMetro {
	return &capxv1.NutanixMetro{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: capxv1.NutanixMetroSpec{
			FailureDomains: []corev1.LocalObjectReference{
				{Name: fd1},
				{Name: fd2},
			},
		},
	}
}

// metroSubnetSpec describes the network attributes a mocked subnet should carry.
type metroSubnetSpec struct {
	subnetType netv4.SubnetType
	networkID  *int
	cidr       *string
}

// metroNClient builds a mock Nutanix client that returns a PE cluster for each
// PE UUID and a subnet (with the given network attributes) for each subnet UUID.
func metroNClient(subnets map[string]metroSubnetSpec) *clientWrapper {
	return &clientWrapper{
		GetClusterByIdFunc: func(
			ctx context.Context,
			uuid *string,
			args ...map[string]any,
		) (*clustermgmtv4.GetClusterApiResponse, error) {
			cluster := clustermgmtv4.NewCluster()
			cluster.ExtId = uuid
			cluster.Name = uuid
			resp := &clustermgmtv4.GetClusterApiResponse{
				ObjectType_: ptr.To("clustermgmt.v4.config.GetClusterApiResponse"),
			}
			if err := resp.SetData(*cluster); err != nil {
				return nil, err
			}
			return resp, nil
		},
		GetSubnetByIdFunc: func(
			ctx context.Context,
			uuid *string,
			args ...map[string]any,
		) (*netv4.GetSubnetApiResponse, error) {
			spec := subnets[*uuid]
			subnet := netv4.NewSubnet()
			subnet.ExtId = uuid
			subnet.Name = uuid
			subnet.SubnetType = spec.subnetType.Ref()
			subnet.NetworkId = spec.networkID
			subnet.IpPrefix = spec.cidr
			resp := &netv4.GetSubnetApiResponse{
				ObjectType_: ptr.To("networking.v4.config.GetSubnetApiResponse"),
			}
			if err := resp.SetData(*subnet); err != nil {
				return nil, err
			}
			return resp, nil
		},
	}
}

func TestNewMetroChecks(t *testing.T) {
	testCases := []struct {
		name                     string
		nutanixClusterConfigSpec *carenv1.NutanixClusterConfigSpec
		machineDeployments       []clusterv1beta2.MachineDeploymentTopology
		objects                  []ctrlclient.Object
		nclient                  client
		expectedChecksCount      int
	}{
		{
			name:                "client not initialized",
			nclient:             nil,
			expectedChecksCount: 0,
		},
		{
			name: "non-metro control plane failure domain",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{"plain-fd"},
					},
				},
			},
			nclient:             &clientWrapper{},
			expectedChecksCount: 0,
		},
		{
			name: "metro control plane failure domain",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{metroFailureDomainPrefix + metroName},
					},
				},
			},
			nclient: &clientWrapper{},
			// 1 per-metro check + 1 cluster PE-scale check.
			expectedChecksCount: 2,
		},
		{
			name: "same metro referenced by control plane and worker is de-duplicated",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{metroFailureDomainPrefix + metroName},
					},
				},
			},
			machineDeployments: []clusterv1beta2.MachineDeploymentTopology{{
				Name:          "md-1",
				FailureDomain: metroFailureDomainPrefix + metroName,
			}},
			nclient: &clientWrapper{},
			// 1 per-metro check + 1 cluster PE-scale check.
			expectedChecksCount: 2,
		},
		{
			name: "two distinct metros add a single-metro check",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{metroFailureDomainPrefix + metroName},
					},
				},
			},
			machineDeployments: []clusterv1beta2.MachineDeploymentTopology{{
				Name:          "md-1",
				FailureDomain: metroFailureDomainPrefix + "metro-2",
			}},
			nclient: &clientWrapper{},
			// 2 per-metro checks + 1 single-metro check + 1 cluster PE-scale check.
			expectedChecksCount: 4,
		},
		{
			name: "metro site failure domain resolves to its metro",
			machineDeployments: []clusterv1beta2.MachineDeploymentTopology{{
				Name:          "md-1",
				FailureDomain: metroSiteFailureDomainPrefix + "site-1",
			}},
			objects: []ctrlclient.Object{
				&capxv1.NutanixMetroSite{
					ObjectMeta: metav1.ObjectMeta{Name: "site-1", Namespace: namespace},
					Spec: capxv1.NutanixMetroSiteSpec{
						MetroRef:               corev1.LocalObjectReference{Name: metroName},
						PreferredFailureDomain: corev1.LocalObjectReference{Name: metroFD1},
					},
				},
			},
			nclient: &clientWrapper{},
			// 1 per-metro check + 1 cluster PE-scale check.
			expectedChecksCount: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(metroScheme())
			if len(tc.objects) > 0 {
				builder = builder.WithObjects(tc.objects...)
			}
			kclient := builder.Build()

			cd := &checkDependencies{
				nutanixClusterConfigSpec: tc.nutanixClusterConfigSpec,
				cluster: &clusterv1beta2.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
					Spec: clusterv1beta2.ClusterSpec{
						Topology: clusterv1beta2.Topology{
							Workers: clusterv1beta2.WorkersTopology{
								MachineDeployments: tc.machineDeployments,
							},
						},
					},
				},
				nclient:   tc.nclient,
				pcVersion: "7.3.0",
				log:       logr.Discard(),
			}
			if tc.nclient != nil {
				cd.kclient = kclient
			}

			checks := newMetroChecks(cd)
			assert.Len(t, checks, tc.expectedChecksCount)
		})
	}
}

func TestMetroCheck(t *testing.T) {
	testCases := []struct {
		name                  string
		objects               []ctrlclient.Object
		nclient               client
		expectedAllowed       bool
		expectedInternalError bool
		expectedCauseMessage  string
	}{
		{
			name:                 "metro not found",
			objects:              nil,
			nclient:              &clientWrapper{},
			expectedAllowed:      false,
			expectedCauseMessage: "was not found",
		},
		{
			name: "valid metro spanning two PEs with consistent VLAN subnets",
			objects: []ctrlclient.Object{
				newMetroObject(metroName, metroFD1, metroFD2),
				newMetroFailureDomain(metroFD1, metroPE1UUID, metroSubnet1),
				newMetroFailureDomain(metroFD2, metroPE2UUID, metroSubnet2),
			},
			nclient: metroNClient(map[string]metroSubnetSpec{
				metroSubnet1: {subnetType: netv4.SUBNETTYPE_VLAN, networkID: ptr.To(100), cidr: ptr.To("10.0.0.0/24")},
				metroSubnet2: {subnetType: netv4.SUBNETTYPE_VLAN, networkID: ptr.To(100), cidr: ptr.To("10.0.0.0/24")},
			}),
			expectedAllowed: true,
		},
		{
			name: "failure domains resolve to the same PE",
			objects: []ctrlclient.Object{
				newMetroObject(metroName, metroFD1, metroFD2),
				newMetroFailureDomain(metroFD1, metroPE1UUID, metroSubnet1),
				newMetroFailureDomain(metroFD2, metroPE1UUID, metroSubnet2),
			},
			nclient: metroNClient(map[string]metroSubnetSpec{
				metroSubnet1: {subnetType: netv4.SUBNETTYPE_VLAN},
				metroSubnet2: {subnetType: netv4.SUBNETTYPE_VLAN},
			}),
			expectedAllowed:      false,
			expectedCauseMessage: "must span exactly 2 distinct Prism Elements",
		},
		{
			name: "subnets reside on different network layers",
			objects: []ctrlclient.Object{
				newMetroObject(metroName, metroFD1, metroFD2),
				newMetroFailureDomain(metroFD1, metroPE1UUID, metroSubnet1),
				newMetroFailureDomain(metroFD2, metroPE2UUID, metroSubnet2),
			},
			nclient: metroNClient(map[string]metroSubnetSpec{
				metroSubnet1: {subnetType: netv4.SUBNETTYPE_VLAN},
				metroSubnet2: {subnetType: netv4.SUBNETTYPE_OVERLAY},
			}),
			expectedAllowed:      false,
			expectedCauseMessage: "reside on multiple network layers",
		},
		{
			name: "multiple subnets per FD with matching subnet sets are allowed",
			objects: []ctrlclient.Object{
				newMetroObject(metroName, metroFD1, metroFD2),
				newMetroFailureDomain(metroFD1, metroPE1UUID, metroSubnet1, metroSubnet2),
				newMetroFailureDomain(metroFD2, metroPE2UUID, metroSubnet3, metroSubnet4),
			},
			nclient: metroNClient(map[string]metroSubnetSpec{
				metroSubnet1: {subnetType: netv4.SUBNETTYPE_VLAN, networkID: ptr.To(100), cidr: ptr.To("10.0.0.0/24")},
				metroSubnet2: {subnetType: netv4.SUBNETTYPE_VLAN, networkID: ptr.To(200), cidr: ptr.To("10.0.1.0/24")},
				metroSubnet3: {subnetType: netv4.SUBNETTYPE_VLAN, networkID: ptr.To(100), cidr: ptr.To("10.0.0.0/24")},
				metroSubnet4: {subnetType: netv4.SUBNETTYPE_VLAN, networkID: ptr.To(200), cidr: ptr.To("10.0.1.0/24")},
			}),
			expectedAllowed: true,
		},
		{
			name: "subnet sets differ across failure domains",
			objects: []ctrlclient.Object{
				newMetroObject(metroName, metroFD1, metroFD2),
				newMetroFailureDomain(metroFD1, metroPE1UUID, metroSubnet1, metroSubnet2),
				newMetroFailureDomain(metroFD2, metroPE2UUID, metroSubnet3, metroSubnet4),
			},
			nclient: metroNClient(map[string]metroSubnetSpec{
				metroSubnet1: {subnetType: netv4.SUBNETTYPE_VLAN, networkID: ptr.To(100), cidr: ptr.To("10.0.0.0/24")},
				metroSubnet2: {subnetType: netv4.SUBNETTYPE_VLAN, networkID: ptr.To(200), cidr: ptr.To("10.0.1.0/24")},
				metroSubnet3: {subnetType: netv4.SUBNETTYPE_VLAN, networkID: ptr.To(100), cidr: ptr.To("10.0.0.0/24")},
				metroSubnet4: {subnetType: netv4.SUBNETTYPE_VLAN, networkID: ptr.To(300), cidr: ptr.To("10.0.2.0/24")},
			}),
			expectedAllowed:      false,
			expectedCauseMessage: "do not match",
		},
		{
			name: "prism element returned without ExtId",
			objects: []ctrlclient.Object{
				newMetroObject(metroName, metroFD1, metroFD2),
				newMetroFailureDomain(metroFD1, metroPE1UUID, metroSubnet1),
				newMetroFailureDomain(metroFD2, metroPE2UUID, metroSubnet2),
			},
			nclient: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]any,
				) (*clustermgmtv4.GetClusterApiResponse, error) {
					cluster := clustermgmtv4.NewCluster()
					cluster.Name = uuid
					// ExtId intentionally left nil.
					resp := &clustermgmtv4.GetClusterApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.GetClusterApiResponse"),
					}
					if err := resp.SetData(*cluster); err != nil {
						return nil, err
					}
					return resp, nil
				},
			},
			expectedAllowed:       false,
			expectedInternalError: true,
			expectedCauseMessage:  "without an ExtId",
		},
		{
			name: "referenced failure domain not found",
			objects: []ctrlclient.Object{
				newMetroObject(metroName, metroFD1, metroFD2),
				newMetroFailureDomain(metroFD1, metroPE1UUID, metroSubnet1),
			},
			nclient: metroNClient(map[string]metroSubnetSpec{
				metroSubnet1: {subnetType: netv4.SUBNETTYPE_VLAN},
			}),
			expectedAllowed:      false,
			expectedCauseMessage: "was not found",
		},
	}

	ctx := context.TODO()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(metroScheme())
			if len(tc.objects) > 0 {
				builder = builder.WithObjects(tc.objects...)
			}
			kclient := builder.Build()

			check := &metroCheck{
				metroName: metroName,
				namespace: namespace,
				field:     field,
				kclient:   kclient,
				nclient:   tc.nclient,
			}

			result := check.Run(ctx)

			assert.Equal(t, tc.expectedAllowed, result.Allowed)
			assert.Equal(t, tc.expectedInternalError, result.InternalError)
			if tc.expectedCauseMessage != "" {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, tc.expectedCauseMessage)
				assert.Equal(t, field, result.Causes[0].Field)
			} else {
				assert.Empty(t, result.Causes)
			}
		})
	}
}

func TestSingleMetroCheck(t *testing.T) {
	testCases := []struct {
		name            string
		metroNames      []string
		expectedAllowed bool
	}{
		{
			name:            "single metro is allowed",
			metroNames:      []string{metroName},
			expectedAllowed: true,
		},
		{
			name:            "multiple metros are rejected",
			metroNames:      []string{metroName, "metro-2"},
			expectedAllowed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			check := &singleMetroCheck{metroNames: tc.metroNames, field: field}
			result := check.Run(context.TODO())

			assert.Equal(t, tc.expectedAllowed, result.Allowed)
			if !tc.expectedAllowed {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, "at most one NutanixMetro")
				assert.Equal(t, field, result.Causes[0].Field)
			} else {
				assert.Empty(t, result.Causes)
			}
		})
	}
}

func TestMetroCheckErrMessage(t *testing.T) {
	check := &metroCheck{
		metroName:  metroName,
		namespace:  namespace,
		field:      field,
		errMessage: ptr.To("boom"),
	}

	result := check.Run(context.TODO())

	assert.False(t, result.Allowed)
	require.NotEmpty(t, result.Causes)
	assert.Contains(t, result.Causes[0].Message, "boom")
}

func TestClusterPrismElementScaleCheck(t *testing.T) {
	const metroFD3 = "metro-fd-3"

	testCases := []struct {
		name                 string
		failureDomainNames   []string
		objects              []ctrlclient.Object
		errMessage           *string
		expectedAllowed      bool
		expectedCauseMessage string
	}{
		{
			name:                 "resolution error message is surfaced",
			errMessage:           ptr.To("boom"),
			expectedAllowed:      false,
			expectedCauseMessage: "boom",
		},
		{
			name:               "cluster spanning exactly two Prism Elements is allowed",
			failureDomainNames: []string{metroFD1, metroFD2},
			objects: []ctrlclient.Object{
				newMetroFailureDomain(metroFD1, metroPE1UUID, metroSubnet1),
				newMetroFailureDomain(metroFD2, metroPE2UUID, metroSubnet2),
			},
			expectedAllowed: true,
		},
		{
			name:               "cluster spanning three Prism Elements is rejected",
			failureDomainNames: []string{metroFD1, metroFD2, metroFD3},
			objects: []ctrlclient.Object{
				newMetroFailureDomain(metroFD1, metroPE1UUID, metroSubnet1),
				newMetroFailureDomain(metroFD2, metroPE2UUID, metroSubnet2),
				newMetroFailureDomain(metroFD3, "metro-pe-3-uuid", "metro-subnet-3-uuid"),
			},
			expectedAllowed:      false,
			expectedCauseMessage: "must span exactly 2 distinct Prism Elements",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(metroScheme())
			if len(tc.objects) > 0 {
				builder = builder.WithObjects(tc.objects...)
			}
			kclient := builder.Build()

			check := &clusterPrismElementScaleCheck{
				failureDomainNames: tc.failureDomainNames,
				namespace:          namespace,
				field:              field,
				kclient:            kclient,
				nclient:            metroNClient(map[string]metroSubnetSpec{}),
				errMessage:         tc.errMessage,
			}

			result := check.Run(context.TODO())

			assert.Equal(t, tc.expectedAllowed, result.Allowed)
			if !tc.expectedAllowed {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, tc.expectedCauseMessage)
			} else {
				assert.Empty(t, result.Causes)
			}
		})
	}
}
