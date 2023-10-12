// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.GenericClusterConfig{}.VariableSchema()),
		false,
		clusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "without a Secret",
			Vals: v1alpha1.GenericClusterConfig{
				ImageRegistries: v1alpha1.ImageRegistries{
					ImageRegistryCredentials: []v1alpha1.ImageRegistryCredentialsResource{
						{
							URL: "http://a.b.c.example.com",
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "with a Secret",
			Vals: v1alpha1.GenericClusterConfig{
				ImageRegistries: v1alpha1.ImageRegistries{
					ImageRegistryCredentials: []v1alpha1.ImageRegistryCredentialsResource{
						{
							URL: "http://a.b.c.example.com",
							Secret: &corev1.ObjectReference{
								Name: "a.b.c.example.com-creds",
							},
						},
					},
				},
			},
		},
	)
}
