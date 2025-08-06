// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package iaminstanceprofile

import (
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/iaminstanceprofile"
)

func NewWorkerPatch() mutation.MetaMutator {
	return iaminstanceprofile.NewAWSIAMInstanceProfileWorkerPatchHandler(
		v1alpha1.WorkerConfigVariableName,
		v1alpha1.EKSVariableName,
		iaminstanceprofile.VariableName,
	)
}
