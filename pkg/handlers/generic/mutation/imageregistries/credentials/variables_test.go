// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var testDefs = []capitest.VariableTestDef{
	{
		Name: "without a credentials secret",
		Vals: v1alpha1.GenericClusterConfigSpec{
			ImageRegistries: []v1alpha1.ImageRegistry{
				{
					URL: "http://a.b.c.example.com",
				},
			},
		},
	}, {
		Name: "with a credentials secret",
		Vals: v1alpha1.GenericClusterConfigSpec{
			ImageRegistries: []v1alpha1.ImageRegistry{
				{
					URL: "https://a.b.c.example.com/a/b/c",
					Credentials: &v1alpha1.RegistryCredentials{
						SecretRef: &v1alpha1.LocalObjectReference{
							Name: "a.b.c.example.com-creds",
						},
					},
				},
			},
		},
	}, {
		Name: "support for multiple image registries",
		Vals: v1alpha1.GenericClusterConfigSpec{
			ImageRegistries: []v1alpha1.ImageRegistry{
				{
					URL: "http://first-image-registry.example.com",
				},
				{
					URL: "http://second-image-registry.example.com",
				},
			},
		},
	}, {
		Name: "invalid registry URL",
		Vals: v1alpha1.GenericClusterConfigSpec{
			ImageRegistries: []v1alpha1.ImageRegistry{
				{
					URL: "unsupportedformat://a.b.c.example.com",
				},
			},
		},
		ExpectError: true,
	}, {
		Name: "registry URL without format",
		Vals: v1alpha1.GenericClusterConfigSpec{
			ImageRegistries: []v1alpha1.ImageRegistry{
				{
					URL: "a.b.c.example.com/a/b/c",
				},
			},
		},
		ExpectError: true,
	},
}

func TestVariableValidation_AWS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_Docker(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_Nutanix(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		testDefs...,
	)
}
