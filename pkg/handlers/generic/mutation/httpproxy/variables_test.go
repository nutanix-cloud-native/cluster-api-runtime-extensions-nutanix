// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.GenericClusterConfig{}.VariableSchema()),
		false,
		clusterconfig.NewVariable,
		// HTTPProxy
		capitest.VariableTestDef{
			Name: "valid proxy config",
			Vals: v1alpha1.GenericClusterConfigSpec{
				Proxy: &v1alpha1.HTTPProxy{
					HTTP:         "http://a.b.c.example.com",
					HTTPS:        "https://a.b.c.example.com",
					AdditionalNo: []string{"d.e.f.example.com"},
				},
			},
		},
	)
}
