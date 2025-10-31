// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomain

import (
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	awsfailuredomain "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/failuredomain"
)

func NewWorkerPatch() mutation.MetaMutator {
	return awsfailuredomain.NewAWSFailureDomainWorkerPatchHandler(
		v1alpha1.WorkerConfigVariableName,
		v1alpha1.EKSVariableName,
		awsfailuredomain.VariableName,
	)
}
