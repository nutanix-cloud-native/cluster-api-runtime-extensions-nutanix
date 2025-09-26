// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package parallelimagepulls

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
		Name: "parallel image pulls unlimited",
		Vals: ptr.To[int32](0),
	}, {
		Name: "parallel image pulls set to 5",
		Vals: ptr.To[int32](5),
	}, {
		Name: "parallel image pulls set to 1",
		Vals: ptr.To[int32](1),
	}, {
		Name:        "parallel image pulls set to -1",
		Vals:        ptr.To[int32](-1),
		ExpectError: true,
	}}

	g := gomega.NewWithT(t)

	for i := range testDefs {
		testDef := &testDefs[i]

		if testDef.Vals != nil {
			clusterConfigVal := updateParallelImagePulls(g, clusterConfig, testDef.Vals.(*int32))
			testDef.Vals = clusterConfigVal
		} else {
			testDef.Vals = clusterConfig
		}
	}

	return testDefs
}

func updateParallelImagePulls[T any](g gomega.Gomega, clusterConfig T, parallelImagePulls *int32) T {
	unmarshalled, err := json.Marshal(clusterConfig)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var unstr map[string]any
	g.Expect(json.Unmarshal(unmarshalled, &unstr)).To(gomega.Succeed())

	if parallelImagePulls != nil {
		err = unstructured.SetNestedField(
			unstr,
			int64(*parallelImagePulls),
			"maxParallelImagePullsPerNode",
		)
	} else {
		err = unstructured.SetNestedField(
			unstr,
			nil,
			"maxParallelImagePullsPerNode",
		)
	}
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
				CustomImage: "fake-docker-image",
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
			Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
				MachineDetails: v1alpha1.NutanixMachineDetails{
					BootType:       capxv1.NutanixBootTypeLegacy,
					VCPUSockets:    2,
					VCPUsPerSocket: 1,
					Image: &capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To("fake-image"),
					},
					Cluster: &capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To("fake-pe-cluster"),
					},
					MemorySize:     resource.MustParse("8Gi"),
					SystemDiskSize: resource.MustParse("40Gi"),
					Subnets: []capxv1.NutanixResourceIdentifier{
						{
							Type: capxv1.NutanixIdentifierName,
							Name: ptr.To("fake-subnet"),
						},
					},
				},
			},
		},
	}
}
