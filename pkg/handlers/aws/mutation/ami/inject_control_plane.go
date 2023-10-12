// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ami

import (
	_ "embed"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	awsclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

func NewControlPlanePatch() *awsAMISpecPatchHandler {
	return newAWSAMISpecPatchHandler(
		clusterconfig.MetaVariableName,
		[]string{
			clusterconfig.MetaControlPlaneConfigName,
			awsclusterconfig.AWSVariableName,
			VariableName,
		},
		selectors.InfrastructureControlPlaneMachines(
			capav1.GroupVersion.Version,
			"AWSMachineTemplate",
		),
	)
}
