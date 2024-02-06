// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

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
			Name: "without a credentials secret",
			Vals: v1alpha1.GenericClusterConfig{
				GlobalImageRegistryMirror: &v1alpha1.GlobalImageRegistryMirror{
					URL: "http://a.b.c.example.com",
				},
			},
		},
		capitest.VariableTestDef{
			Name: "with a credentials CA secret",
			Vals: v1alpha1.GenericClusterConfig{
				GlobalImageRegistryMirror: &v1alpha1.GlobalImageRegistryMirror{
					URL: "http://a.b.c.example.com",
					Credentials: &v1alpha1.RegistryCredentials{
						SecretRef: &corev1.ObjectReference{
							Name: "a.b.c.example.com-ca-cert-creds",
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid mirror registry URL",
			Vals: v1alpha1.GenericClusterConfig{
				GlobalImageRegistryMirror: &v1alpha1.GlobalImageRegistryMirror{
					URL: "unsupportedformat://a.b.c.example.com",
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "mirror URL without format",
			Vals: v1alpha1.GenericClusterConfig{
				GlobalImageRegistryMirror: &v1alpha1.GlobalImageRegistryMirror{
					URL: "a.b.c.example.com/a/b/c",
				},
			},
			ExpectError: true,
		},
	)
}
