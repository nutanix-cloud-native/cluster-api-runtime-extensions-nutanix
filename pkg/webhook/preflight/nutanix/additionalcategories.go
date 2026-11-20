// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"

	prismv4 "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/config"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type additionalCategoriesCheck struct {
	machineDetails *carenv1.NutanixMachineDetails
	field          string
	nclient        client
	pcVersion      string
	projectRef     *capxv1.NutanixResourceIdentifier
}

func (c *additionalCategoriesCheck) Name() string {
	return "NutanixAdditionalCategories"
}

func (c *additionalCategoriesCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}
	if c.machineDetails == nil || len(c.machineDetails.AdditionalCategories) == 0 {
		return result
	}

	scopeCtx, err := resolveProjectScopeContext(ctx, c.nclient, c.pcVersion, c.projectRef)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to resolve project scope for additional category validation: %s. Please retry.",
				err,
			),
			Field: c.field + ".project",
		})
		return result
	}

	for _, category := range c.machineDetails.AdditionalCategories {
		key := strings.TrimSpace(category.Key)
		value := strings.TrimSpace(category.Value)
		if key == "" || value == "" {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Invalid additional category key/value pair %q:%q. Both key and value are required.",
					category.Key,
					category.Value,
				),
				Field: c.field + ".additionalCategories",
			})
			continue
		}

		var categories []prismv4.Category
		var lookupErr error
		if scopeCtx != nil {
			categories, lookupErr = getCategoriesInProject(ctx, c.nclient, key, value, scopeCtx.projectUUID)
		} else {
			categories, lookupErr = getCategories(ctx, c.nclient, key, value)
		}
		if lookupErr != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to validate additional category %q:%q: %s. This is usually a temporary error. Please retry.",
					key,
					value,
					lookupErr,
				),
				Field: c.field + ".additionalCategories",
			})
			continue
		}

		switch len(categories) {
		case 0:
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"No category %q:%q found in Prism Central "+
						"for the effective project scope. "+
						"Please contact your Prism Central administrator and retry.",
					key,
					value,
				),
				Field: c.field + ".additionalCategories",
			})
		case 1:
			continue
		default:
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Found %d categories matching %q:%q. There must be exactly one category match.",
					len(categories),
					key,
					value,
				),
				Field: c.field + ".additionalCategories",
			})
		}
	}

	return result
}

func newAdditionalCategoriesChecks(cd *checkDependencies) []preflight.Check {
	checks := []preflight.Check{}
	if cd == nil || cd.nclient == nil || cd.pcVersion == "" {
		return checks
	}

	if cd.nutanixClusterConfigSpec != nil && cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		checks = append(checks, &additionalCategoriesCheck{
			machineDetails: &cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails,
			field:          controlPlaneMachineDetailsField,
			nclient:        cd.nclient,
			pcVersion:      cd.pcVersion,
			projectRef:     cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails.Project,
		})
	}

	for mdName, workerSpec := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if workerSpec == nil || workerSpec.Nutanix == nil {
			continue
		}
		checks = append(checks, &additionalCategoriesCheck{
			machineDetails: &workerSpec.Nutanix.MachineDetails,
			field: fmt.Sprintf(
				"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=='%s'].value.nutanix.machineDetails", //nolint:lll // Field is long.
				mdName,
				carenv1.WorkerConfigVariableName,
			),
			nclient:    cd.nclient,
			pcVersion:  cd.pcVersion,
			projectRef: workerSpec.Nutanix.MachineDetails.Project,
		})
	}

	return checks
}

func getCategories(
	ctx context.Context,
	nclient client,
	key string,
	value string,
) ([]prismv4.Category, error) {
	filter := fmt.Sprintf("key eq '%s' and value eq '%s'", escapeODataString(key), escapeODataString(value))
	categories, err := nclient.ListCategoriesByFilter(ctx, filter)
	if err != nil {
		return nil, err
	}
	return categories, nil
}

func getCategoriesInProject(
	ctx context.Context,
	nclient client,
	key string,
	value string,
	projectUUID string,
) ([]prismv4.Category, error) {
	// Prefer project-owned category over shared matches for same key:value.
	ownedFilter := fmt.Sprintf(
		"key eq '%s' and value eq '%s' and projectExtId eq '%s'",
		escapeODataString(key),
		escapeODataString(value),
		escapeODataString(projectUUID),
	)
	categories, err := nclient.ListCategoriesByFilter(ctx, ownedFilter)
	if err != nil {
		return nil, err
	}
	if len(categories) > 0 {
		return categories, nil
	}

	sharedFilter := fmt.Sprintf(
		"key eq '%s' and value eq '%s' and "+
			"(sharedWithProjects/any(a:a eq '%s') or "+
			"isSharedWithAllProjects eq true)",
		escapeODataString(key),
		escapeODataString(value),
		escapeODataString(projectUUID),
	)
	categories, err = nclient.ListCategoriesByFilter(ctx, sharedFilter)
	if err != nil {
		return nil, err
	}
	return categories, nil
}
