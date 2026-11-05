// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package volumes

import (
	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
)

func NewWorkerPatch() *awsVolumesSpecPatchHandler {
	return NewAWSVolumesSpecPatchHandler(
		v1alpha1.WorkerConfigVariableName,
		[]string{
			v1alpha1.AWSVariableName,
			VariableName,
		},
		selectors.InfrastructureWorkerMachineTemplates(
			capav1.GroupVersion.Version,
			"AWSMachineTemplate",
		),
	)
}
