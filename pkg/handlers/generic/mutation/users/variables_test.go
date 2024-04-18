// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package users

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
		capitest.VariableTestDef{
			Name: "valid users",
			Vals: v1alpha1.GenericClusterConfigSpec{
				Users: []v1alpha1.User{
					{
						Name:           "complete",
						HashedPassword: "password",
						SSHAuthorizedKeys: []string{
							"key1",
							"key2",
						},
						Sudo: "ALL=(ALL) NOPASSWD:ALL",
					},
					{
						Name: "onlyname",
					},
				},
			},
		},
	)
}
