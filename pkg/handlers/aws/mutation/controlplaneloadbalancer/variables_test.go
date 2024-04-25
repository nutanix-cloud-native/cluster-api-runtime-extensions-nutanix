// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneloadbalancer

import (
	"testing"

	"k8s.io/utils/ptr"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified internet-facing scheme",
			Vals: v1alpha1.AWSClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					ControlPlaneLoadBalancer: &v1alpha1.AWSLoadBalancerSpec{
						Scheme: &capav1.ELBSchemeInternetFacing,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "specified internal scheme",
			Vals: v1alpha1.AWSClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					ControlPlaneLoadBalancer: &v1alpha1.AWSLoadBalancerSpec{
						Scheme: &capav1.ELBSchemeInternal,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "specified invalid scheme",
			Vals: v1alpha1.AWSClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					ControlPlaneLoadBalancer: &v1alpha1.AWSLoadBalancerSpec{
						Scheme: ptr.To(capav1.ELBScheme("invalid")),
					},
				},
			},
			ExpectError: true,
		},
	)
}
