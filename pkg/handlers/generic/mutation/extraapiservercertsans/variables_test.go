// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

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

func TestVariableValidation_AWS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		[]capitest.VariableTestDef{{
			Name: "single valid SAN",
			Vals: v1alpha1.AWSClusterConfigSpec{
				ExtraAPIServerCertSANs: []string{"a.b.c.example.com"},
			},
		}, {
			Name: "single invalid SAN",
			Vals: v1alpha1.AWSClusterConfigSpec{
				ExtraAPIServerCertSANs: []string{"invalid:san"},
			},
			ExpectError: true,
		}, {
			Name: "duplicate valid SANs",
			Vals: v1alpha1.AWSClusterConfigSpec{
				ExtraAPIServerCertSANs: []string{
					"a.b.c.example.com",
					"a.b.c.example.com",
				},
			},
			ExpectError: true,
		}}...,
	)
}

func TestVariableValidation_Docker(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		[]capitest.VariableTestDef{{
			Name: "single valid SAN",
			Vals: v1alpha1.DockerClusterConfigSpec{
				ExtraAPIServerCertSANs: []string{"a.b.c.example.com"},
			},
		}, {
			Name: "single invalid SAN",
			Vals: v1alpha1.DockerClusterConfigSpec{
				ExtraAPIServerCertSANs: []string{"invalid:san"},
			},
			ExpectError: true,
		}, {
			Name: "duplicate valid SANs",
			Vals: v1alpha1.DockerClusterConfigSpec{
				ExtraAPIServerCertSANs: []string{
					"a.b.c.example.com",
					"a.b.c.example.com",
				},
			},
			ExpectError: true,
		}}...,
	)
}

func TestVariableValidation_Nutanix(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		[]capitest.VariableTestDef{{
			Name: "single valid SAN",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				ExtraAPIServerCertSANs: []string{"a.b.c.example.com"},
			},
		}, {
			Name: "single invalid SAN",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				ExtraAPIServerCertSANs: []string{"invalid:san"},
			},
			ExpectError: true,
		}, {
			Name: "duplicate valid SANs",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				ExtraAPIServerCertSANs: []string{
					"a.b.c.example.com",
					"a.b.c.example.com",
				},
			},
			ExpectError: true,
		}}...,
	)
}
