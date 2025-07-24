// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package generic

import (
	"context"
	"fmt"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type configurationCheck struct {
	result preflight.CheckResult
}

func (c *configurationCheck) Name() string {
	return "GenericConfiguration"
}

func (c *configurationCheck) Run(_ context.Context) preflight.CheckResult {
	return c.result
}

func newConfigurationCheck(
	cd *checkDependencies,
) preflight.Check {
	genericClusterConfigSpec := &carenv1.GenericClusterConfigSpec{}
	configurationCheck := &configurationCheck{
		result: preflight.CheckResult{
			Allowed: true,
		},
	}
	err := variables.UnmarshalClusterVariable(
		variables.GetClusterVariableByName(
			carenv1.ClusterConfigVariableName,
			cd.cluster.Spec.Topology.Variables,
		),
		genericClusterConfigSpec,
	)
	if err != nil {
		configurationCheck.result.Allowed = false
		configurationCheck.result.InternalError = true
		configurationCheck.result.Causes = append(configurationCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to unmarshal cluster variable %q: %s. Review the Cluster.", ///nolint:lll // Message is long.
					carenv1.ClusterConfigVariableName,
					err,
				),
				Field: "$.spec.topology.variables[?@.name==\"clusterConfig\"]",
			},
		)
	}
	cd.genericClusterConfigSpec = genericClusterConfigSpec
	return configurationCheck
}
