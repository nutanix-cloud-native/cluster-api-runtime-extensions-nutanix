// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package instancetype

import (
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	awsinstancetype "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/instancetype"
)

func NewWorkerPatch() mutation.MetaMutator {
	return awsinstancetype.NewAWSInstanceTypeWorkerPatchHandler(
		v1alpha1.WorkerConfigVariableName,
		v1alpha1.EKSVariableName,
		awsinstancetype.VariableName,
	)
}
