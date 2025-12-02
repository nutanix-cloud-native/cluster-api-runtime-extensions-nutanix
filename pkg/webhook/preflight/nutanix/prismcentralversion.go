// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
	nutanixutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/nutanix/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/skip"
)

const (
	prismCentralEndpointFieldPath    = "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.nutanix.prismCentralEndpoint" ///nolint:lll // Field path is intentionally long.
	minSupportedPrismCentralVersion  = "7.3"
	internalPrismCentralVersionLabel = "master"
)

type prismCentralVersionCheck struct {
	result preflight.CheckResult
}

func newPrismCentralVersionCheck(ctx context.Context, cd *checkDependencies) preflight.Check {
	cd.log.V(5).Info("Initializing Prism Central version check")

	check := &prismCentralVersionCheck{
		result: preflight.CheckResult{
			Allowed: true,
		},
	}

	if cd == nil {
		return check
	}

	// If check is skipped, set a fake version to allow other checks to run.
	if isPCVersionCheckSkipped(cd.cluster) {
		cd.pcVersion = "skipped"
		return check
	}

	// If already validated, skip the API call.
	if cd.pcVersion != "" {
		return check
	}

	if cd.nclient == nil {
		return check
	}

	version, err := cd.nclient.GetPrismCentralVersion(ctx)
	if err != nil {
		check.result.Allowed = false
		check.result.InternalError = true
		check.result.Causes = append(check.result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to get Prism Central version: %s. This is usually a temporary error. Please retry.",
				err,
			),
			Field: prismCentralEndpointFieldPath,
		})
		// pcVersion remains empty on failure
		return check
	}

	lowerVersion := strings.ToLower(strings.TrimSpace(version))
	if strings.Contains(lowerVersion, internalPrismCentralVersionLabel) {
		cd.pcVersion = version
		return check
	}

	cleanVersion := nutanixutils.CleanPCVersion(lowerVersion)
	if cleanVersion == "" {
		check.result.Allowed = false
		check.result.InternalError = false
		check.result.Causes = append(check.result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Prism Central reported version %q, which is not a valid version. Upgrade Prism Central to %s or later, wait for the upgrade to finish, then retry.", ///nolint:lll // Message includes full upgrade instruction.
				version,
				minSupportedPrismCentralVersion,
			),
			Field: prismCentralEndpointFieldPath,
		})
		// pcVersion remains empty on failure
		return check
	}

	if nutanixutils.ComparePCVersions(cleanVersion, minSupportedPrismCentralVersion) == -1 {
		check.result.Allowed = false
		check.result.Causes = append(check.result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Prism Central version %q is older than the minimum supported version %s. Upgrade Prism Central to %s or later, wait for the upgrade to finish, then retry.", ///nolint:lll // Message includes version and upgrade guidance.
				version,
				minSupportedPrismCentralVersion,
				minSupportedPrismCentralVersion,
			),
			Field: prismCentralEndpointFieldPath,
		})
		// pcVersion remains empty on failure
		return check
	}

	cd.pcVersion = version
	return check
}

func (c *prismCentralVersionCheck) Name() string {
	return "NutanixPrismCentralVersion"
}

func (c *prismCentralVersionCheck) Run(_ context.Context) preflight.CheckResult {
	return c.result
}

func isPCVersionCheckSkipped(cluster *clusterv1.Cluster) bool {
	if cluster == nil {
		return false
	}

	skipEvaluator := skip.New(cluster)
	return skipEvaluator.For("NutanixPrismCentralVersion")
}
