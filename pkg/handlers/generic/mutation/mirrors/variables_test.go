// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

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

var testDefs = []capitest.VariableTestDef{{
	Name: "without a credentials secret",
	Vals: v1alpha1.GenericClusterConfigSpec{
		GlobalImageRegistryMirror: &v1alpha1.GlobalImageRegistryMirror{
			URL: "http://a.b.c.example.com",
		},
	},
}, {
	Name: "with a credentials CA secret",
	Vals: v1alpha1.GenericClusterConfigSpec{
		GlobalImageRegistryMirror: &v1alpha1.GlobalImageRegistryMirror{
			URL: "http://a.b.c.example.com",
			Credentials: &v1alpha1.RegistryCredentials{
				SecretRef: &v1alpha1.LocalObjectReference{
					Name: "a.b.c.example.com-ca-cert-creds",
				},
			},
		},
	},
}, {
	Name: "invalid mirror registry URL",
	Vals: v1alpha1.GenericClusterConfigSpec{
		GlobalImageRegistryMirror: &v1alpha1.GlobalImageRegistryMirror{
			URL: "unsupportedformat://a.b.c.example.com",
		},
	},
	ExpectError: true,
}, {
	Name: "mirror URL without format",
	Vals: v1alpha1.GenericClusterConfigSpec{
		GlobalImageRegistryMirror: &v1alpha1.GlobalImageRegistryMirror{
			URL: "a.b.c.example.com/a/b/c",
		},
	},
	ExpectError: true,
}}

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
