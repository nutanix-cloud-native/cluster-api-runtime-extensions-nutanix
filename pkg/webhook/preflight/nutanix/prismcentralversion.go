// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
	nutanixutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/nutanix/utils"
)

const (
	prismCentralEndpointFieldPath    = "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.nutanix.prismCentralEndpoint" ///nolint:lll // Field path is intentionally long.
	minSupportedPrismCentralVersion  = "7.3"
	internalPrismCentralVersionLabel = "master"
)

type prismCentralVersionCheck struct {
	cd *checkDependencies
}

func newPrismCentralVersionCheck(cd *checkDependencies) preflight.Check {
	cd.log.V(5).Info("Initializing Prism Central version check")
	return &prismCentralVersionCheck{
		cd: cd,
	}
}

func (c *prismCentralVersionCheck) Name() string {
	return "NutanixPrismCentralVersion"
}

func (c *prismCentralVersionCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: true,
	}

	if c.cd == nil || c.cd.nclient == nil {
		return result
	}

	version, err := c.cd.nclient.GetPrismCentralVersion(ctx)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to get Prism Central version: %s. This is usually a temporary error. Please retry.",
				err,
			),
			Field: prismCentralEndpointFieldPath,
		})
		return result
	}

	lowerVersion := strings.ToLower(strings.TrimSpace(version))
	if strings.Contains(lowerVersion, internalPrismCentralVersionLabel) {
		return result
	}

	cleanVersion := nutanixutils.CleanPCVersion(lowerVersion)
	if cleanVersion == "" {
		result.Allowed = false
		result.InternalError = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Prism Central reported version %q, which is not a valid version. Upgrade Prism Central to %s or later, wait for the upgrade to finish, then retry.", ///nolint:lll // Message includes full upgrade instruction.
				version,
				minSupportedPrismCentralVersion,
			),
			Field: prismCentralEndpointFieldPath,
		})
		return result
	}

	if nutanixutils.ComparePCVersions(cleanVersion, minSupportedPrismCentralVersion) == -1 {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Prism Central version %q is older than the minimum supported version %s. Upgrade Prism Central to %s or later, wait for the upgrade to finish, then retry.", ///nolint:lll // Message includes version and upgrade guidance.
				version,
				minSupportedPrismCentralVersion,
				minSupportedPrismCentralVersion,
			),
			Field: prismCentralEndpointFieldPath,
		})
		return result
	}

	return result
}
