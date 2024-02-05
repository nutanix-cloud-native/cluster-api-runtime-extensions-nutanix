// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneloadbalancer

import (
	"testing"

	"k8s.io/utils/ptr"

	capav1 "github.com/d2iq-labs/capi-runtime-extensions/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.ClusterConfigSpec{AWS: &v1alpha1.AWSSpec{}}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified internet-facing scheme",
			Vals: v1alpha1.ClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					ControlPlaneLoadBalancer: &v1alpha1.AWSLoadBalancerSpec{
						Scheme: &capav1.ELBSchemeInternetFacing,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "specified internal scheme",
			Vals: v1alpha1.ClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					ControlPlaneLoadBalancer: &v1alpha1.AWSLoadBalancerSpec{
						Scheme: &capav1.ELBSchemeInternal,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "specified invalid scheme",
			Vals: v1alpha1.ClusterConfigSpec{
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
