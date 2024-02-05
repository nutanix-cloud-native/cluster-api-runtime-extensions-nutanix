// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package securitygroups

import (
	capav1 "github.com/d2iq-labs/capi-runtime-extensions/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/workerconfig"
)

func NewWorkerPatch() *awsSecurityGroupSpecPatchHandler {
	return newAWSSecurityGroupSpecPatchHandler(
		workerconfig.MetaVariableName,
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
