// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "valid",
			Vals: v1alpha1.DockerClusterConfigSpec{
				ControlPlane: &v1alpha1.DockerControlPlaneSpec{
					Docker: &v1alpha1.DockerNodeSpec{
						CustomImage: ptr.To("docker.io/some/image:v2.3.4"),
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid",
			Vals: v1alpha1.DockerClusterConfigSpec{
				ControlPlane: &v1alpha1.DockerControlPlaneSpec{
					Docker: &v1alpha1.DockerNodeSpec{
						CustomImage: ptr.To("this.is.not.valid?"),
					},
				},
			},
			ExpectError: true,
		},
	)
}
