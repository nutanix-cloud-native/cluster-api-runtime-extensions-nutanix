// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeproxymode

import (
	"encoding/json"
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func testDefs[T any](t *testing.T, clusterConfig T) []capitest.VariableTestDef {
	t.Helper()

	testDefs := []capitest.VariableTestDef{{
		Name: "unset",
		Vals: nil,
	}, {
		Name: "kube-proxy iptables mode",
		Vals: v1alpha1.KubeProxyModeIPTables,
	}, {
		Name: "kube-proxy nftables mode",
		Vals: v1alpha1.KubeProxyModeNFTables,
	}, {
		Name:    "kube-proxy mode unchanged from iptables on update",
		Vals:    v1alpha1.KubeProxyModeIPTables,
		OldVals: v1alpha1.KubeProxyModeIPTables,
	}, {
		Name:    "kube-proxy mode unchanged from nftables on update",
		Vals:    v1alpha1.KubeProxyModeNFTables,
		OldVals: v1alpha1.KubeProxyModeNFTables,
	}, {
		Name:        "kube-proxy mode changed from iptables to nftables on update",
		Vals:        v1alpha1.KubeProxyModeIPTables,
		OldVals:     v1alpha1.KubeProxyModeNFTables,
		ExpectError: true,
	}, {
		Name:        "kube-proxy mode changed from nftables to iptables on update",
		Vals:        v1alpha1.KubeProxyModeNFTables,
		OldVals:     v1alpha1.KubeProxyModeIPTables,
		ExpectError: true,
	}}

	g := gomega.NewWithT(t)

	for i := range testDefs {
		testDef := &testDefs[i]

		if testDef.Vals != nil {
			clusterConfigVal := updateKubeProxyMode(g, clusterConfig, testDef.Vals.(v1alpha1.KubeProxyMode))
			testDef.Vals = clusterConfigVal
		} else {
			testDef.Vals = clusterConfig
		}

		if testDef.OldVals != nil {
			oldClusterConfigVal := updateKubeProxyMode(g, clusterConfig, testDef.OldVals.(v1alpha1.KubeProxyMode))
			testDef.OldVals = oldClusterConfigVal
		}
	}

	return testDefs
}

func updateKubeProxyMode[T any](g gomega.Gomega, clusterConfig T, kubeProxyMode v1alpha1.KubeProxyMode) T {
	unmarshalled, err := json.Marshal(clusterConfig)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var unstr map[string]any
	g.Expect(json.Unmarshal(unmarshalled, &unstr)).To(gomega.Succeed())

	err = unstructured.SetNestedField(
		unstr,
		string(kubeProxyMode),
		"kubeProxy",
		"mode",
	)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	unmarshalled, err = json.Marshal(unstr)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var clusterConfigVal T
	g.Expect(json.Unmarshal(unmarshalled, &clusterConfigVal)).To(gomega.Succeed())

	return clusterConfigVal
}

func TestVariableValidation_AWS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		testDefs(t, minimalAWSClusterConfigSpec())...,
	)
}

func minimalAWSClusterConfigSpec() v1alpha1.AWSClusterConfigSpec {
	return v1alpha1.AWSClusterConfigSpec{
		ControlPlane: &v1alpha1.AWSControlPlaneSpec{
			AWS: &v1alpha1.AWSControlPlaneNodeSpec{
				InstanceType: "t3.medium",
			},
		},
	}
}

func TestVariableValidation_Docker(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		testDefs(t, minimalDockerClusterConfigSpec())...,
	)
}

func minimalDockerClusterConfigSpec() v1alpha1.DockerClusterConfigSpec {
	return v1alpha1.DockerClusterConfigSpec{
		ControlPlane: &v1alpha1.DockerControlPlaneSpec{
			Docker: &v1alpha1.DockerNodeSpec{
				CustomImage: ptr.To("fake-docker-image"),
			},
		},
	}
}

func TestVariableValidation_Nutanix(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		testDefs(t, minimalNutanixClusterConfigSpec())...,
	)
}

func minimalNutanixClusterConfigSpec() v1alpha1.NutanixClusterConfigSpec {
	return v1alpha1.NutanixClusterConfigSpec{
		ControlPlane: &v1alpha1.NutanixControlPlaneSpec{
			Nutanix: &v1alpha1.NutanixNodeSpec{
				MachineDetails: v1alpha1.NutanixMachineDetails{
					BootType:       capxv1.NutanixBootTypeLegacy,
					VCPUSockets:    2,
					VCPUsPerSocket: 1,
					Image: &capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To("fake-image"),
					},
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To("fake-pe-cluster"),
					},
					MemorySize:     resource.MustParse("8Gi"),
					SystemDiskSize: resource.MustParse("40Gi"),
					Subnets:        []capxv1.NutanixResourceIdentifier{},
				},
			},
		},
	}
}
