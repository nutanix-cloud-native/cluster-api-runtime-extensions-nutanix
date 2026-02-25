// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ntp

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	eksclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var testDefs = []capitest.VariableTestDef{
	{
		Name: "valid: unset NTP configuration",
		Vals: v1alpha1.GenericClusterConfigSpec{},
	},
	{
		Name: "valid: single NTP server",
		Vals: v1alpha1.GenericClusterConfigSpec{
			NTP: &v1alpha1.NTP{
				Servers: []string{"pool.ntp.org"},
			},
		},
	},
	{
		Name: "valid: multiple, unique NTP servers",
		Vals: v1alpha1.GenericClusterConfigSpec{
			NTP: &v1alpha1.NTP{
				Servers: []string{"time.aws.com", "time.google.com", "pool.ntp.org", "1.2.3.4", "2001:db8::1"},
			},
		},
	},
	{
		Name: "invalid: empty servers array",
		Vals: v1alpha1.GenericClusterConfigSpec{
			NTP: &v1alpha1.NTP{
				Servers: []string{},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid: duplicate NTP servers, case-insensitive",
		Vals: v1alpha1.GenericClusterConfigSpec{
			NTP: &v1alpha1.NTP{
				Servers: []string{"time.aws.com", "TIME.AWS.COM"},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid: server is not a valid IP address or a valid domain name",
		Vals: v1alpha1.GenericClusterConfigSpec{
			NTP: &v1alpha1.NTP{
				Servers: []string{"invalid:server"},
			},
		},
		ExpectError: true,
	},
	{
		Name: "valid: server is an IPv4 address",
		Vals: v1alpha1.GenericClusterConfigSpec{
			NTP: &v1alpha1.NTP{
				Servers: []string{"1.1.1.1"},
			},
		},
	},
	{
		Name: "valid: server is an IPv6 address",
		Vals: v1alpha1.GenericClusterConfigSpec{
			NTP: &v1alpha1.NTP{
				Servers: []string{"2001:db8::1"},
			},
		},
	},
	{
		Name: "valid: server is a domain name",
		Vals: v1alpha1.GenericClusterConfigSpec{
			NTP: &v1alpha1.NTP{
				Servers: []string{
					"hostname",              // Hostname is allowed.
					"example.com",           // Domain is allowed.
					"subdomain.example.com", // Subdomain is allowed.
					"trailing.com.",         // Trailing dot is allowed.
				},
			},
		},
	},
}

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

func TestVariableValidation_EKS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.EKSClusterConfig{}.VariableSchema()),
		true,
		eksclusterconfig.NewVariable,
		testDefs...,
	)
}
