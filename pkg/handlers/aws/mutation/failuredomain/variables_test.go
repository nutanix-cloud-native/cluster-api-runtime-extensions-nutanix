// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomain

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/workerconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.WorkerConfigVariableName,
		ptr.To(v1alpha1.AWSWorkerNodeConfig{}.VariableSchema()),
		false,
		awsworkerconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified failure domain",
			Vals: v1alpha1.AWSWorkerNodeConfigSpec{
				AWS: &v1alpha1.AWSWorkerNodeSpec{
					FailureDomain: "us-west-2a",
				},
			},
		},
	)
}
