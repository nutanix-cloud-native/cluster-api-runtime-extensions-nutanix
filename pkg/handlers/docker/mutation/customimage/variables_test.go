// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	dockerclusterconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.ClusterConfigSpec{Docker: &v1alpha1.DockerSpec{}}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "valid",
			Vals: v1alpha1.ClusterConfigSpec{
				ControlPlane: &v1alpha1.NodeConfigSpec{
					Docker: &v1alpha1.DockerNodeSpec{
						CustomImage: ptr.To(v1alpha1.OCIImage("docker.io/some/image:v2.3.4")),
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid",
			Vals: v1alpha1.ClusterConfigSpec{
				ControlPlane: &v1alpha1.NodeConfigSpec{
					Docker: &v1alpha1.DockerNodeSpec{
						CustomImage: ptr.To(v1alpha1.OCIImage("this.is.not.valid?")),
					},
				},
			},
			ExpectError: true,
		},
	)
}
