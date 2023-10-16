// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

import (
	"testing"

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
			Name: "single valid SAN",
			Vals: v1alpha1.GenericClusterConfig{
				ExtraAPIServerCertSANs: v1alpha1.ExtraAPIServerCertSANs{"a.b.c.example.com"},
			},
		},
		capitest.VariableTestDef{
			Name: "single invalid SAN",
			Vals: v1alpha1.GenericClusterConfig{
				ExtraAPIServerCertSANs: v1alpha1.ExtraAPIServerCertSANs{"invalid:san"},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "duplicate valid SANs",
			Vals: v1alpha1.GenericClusterConfig{
				ExtraAPIServerCertSANs: v1alpha1.ExtraAPIServerCertSANs{
					"a.b.c.example.com",
					"a.b.c.example.com",
				},
			},
			ExpectError: true,
		},
	)
}
