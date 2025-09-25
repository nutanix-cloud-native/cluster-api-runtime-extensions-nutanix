// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var testDefs = []capitest.VariableTestDef{{
	Name: "unset",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Etcd: &v1alpha1.Etcd{},
	},
}, {
	Name: "set with valid image values",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Etcd: &v1alpha1.Etcd{
			Image: &v1alpha1.Image{
				Repository: "my-registry.io/my-org/my-repo",
				Tag:        "v3.5.99_custom.0",
			},
		},
	},
}, {
	Name: "set with invalid image repository",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Etcd: &v1alpha1.Etcd{
			Image: &v1alpha1.Image{
				Repository: "https://this.should.not.have.a.scheme",
			},
		},
	},
	ExpectError: true,
}, {
	Name: "set with invalid image tag",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Etcd: &v1alpha1.Etcd{
			Image: &v1alpha1.Image{
				Tag: "this:is:not:a:valid:tag",
			},
		},
	},
	ExpectError: true,
}}

func TestVariableValidation_AWS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_Docker(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_Nutanix(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		testDefs...,
	)
}
