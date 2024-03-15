// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		MetaVariableName,
		ptr.To(v1alpha1.AllProvidersSpec{}.VariableSchema()),
		false,
		NewVariable,
		capitest.VariableTestDef{
			Name: "valid config",
			Vals: v1alpha1.AllProvidersSpec{
				Proxy: &v1alpha1.HTTPProxy{
					HTTP:         "http://a.b.c.example.com",
					HTTPS:        "https://a.b.c.example.com",
					AdditionalNo: []string{"d.e.f.example.com"},
				},
				ExtraAPIServerCertSANs: v1alpha1.ExtraAPIServerCertSANs{"a.b.c.example.com"},
			},
		},
	)
}
