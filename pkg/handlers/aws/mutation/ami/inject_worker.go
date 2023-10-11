// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package ami

import (
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	awsclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/workerconfig"
)

func NewWorkerPatch() *awsAMISpecPatchHandler {
	return newAWSAMISpecPatchHandler(
		workerconfig.MetaVariableName,
		[]string{
			awsclusterconfig.AWSVariableName,
			VariableName,
		},
		selectors.InfrastructureWorkerMachineTemplates(capav1.GroupVersion.Version, "AWSMachineTemplate"),
	)
}
