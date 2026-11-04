// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"

	multidomainv4 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"

	"github.com/nutanix-cloud-native/prism-go-client/converged"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
)

type userScope string

const (
	userScopeNonProjectScoped userScope = "non-project-scoped"
	userScopeProjectScoped    userScope = "project-scoped"
)

type projectScopeContext struct {
	scope       userScope
	projectUUID string
}

func resolveProjectScopeContext(
	ctx context.Context,
	nclient client,
	pcVersion string,
	projectRef *capxv1.NutanixResourceIdentifier,
) (*projectScopeContext, error) {
	if !nutanixprojectpolicy.PCVersionAtLeast76(pcVersion) {
		return nil, nil
	}
	if nclient == nil {
		return nil, fmt.Errorf("nutanix client is not initialized for PC %s project-scope validation", pcVersion)
	}

	scope := userScopeNonProjectScoped
	defaultProject, err := nclient.ResolvePCDefaultProject(ctx, pcVersion)
	if err != nil {
		if isProjectScopeAccessError(err) {
			scope = userScopeProjectScoped
		} else {
			return nil, err
		}
	}

	var projectUUID string
	switch {
	case projectRef != nil:
		resolved, err := nclient.ResolveProject(ctx, projectRef, pcVersion)
		if err != nil {
			return nil, err
		}
		projectUUID = strings.TrimSpace(resolved.UUID)
	case defaultProject != nil:
		projectUUID = strings.TrimSpace(defaultProject.UUID)
	default:
		return nil, fmt.Errorf("cannot resolve effective project scope on PC %s", pcVersion)
	}

	if projectUUID == "" {
		return nil, fmt.Errorf("resolved project UUID is empty")
	}

	if defaultProject == nil || projectUUID != defaultProject.UUID {
		scope = userScopeProjectScoped
	}

	return &projectScopeContext{
		scope:       scope,
		projectUUID: projectUUID,
	}, nil
}

// isProjectScopeAccessError reports whether the error from resolving the Prism Central default
// project indicates a project-scoped (Projects 2.0) user rather than a hard failure.
//
// resolveProjectScopeContext only ever hands this the error from ResolvePCDefaultProject, which the
// converged SDK (>= v0.8.2) always returns as a classified *converged.APIError: HTTP 403 - and the
// RBAC-hidden default-project case (HTTP 200 + empty list) - map to ErrUnauthorized, while HTTP 401
// maps to ErrUnauthenticated. converged.Is* use errors.Is, which unwraps any fmt wrapping, so no
// HTTP status-code or error-string fallbacks are required.
func isProjectScopeAccessError(err error) bool {
	switch {
	case converged.IsUnauthorized(err):
		// Authenticated but not authorized to read cluster-scoped resources such as the Prism
		// Central default project: the project-scoped signal.
		return true
	case converged.IsUnauthenticated(err):
		// A genuine authentication failure (missing/invalid credentials, HTTP 401) is not a
		// scoping signal: fail the check rather than silently switch to the project-scoped path.
		return false
	default:
		return false
	}
}

func listResourceGroupsForProject(
	ctx context.Context,
	nclient client,
	projectUUID string,
) ([]multidomainv4.ResourceGroup, error) {
	filter := fmt.Sprintf("projectExtId eq '%s'", strings.TrimSpace(projectUUID))
	resourceGroups, err := nclient.ListResourceGroups(ctx, converged.WithFilter(filter))
	if err != nil {
		return nil, err
	}
	if len(resourceGroups) == 0 {
		return nil, fmt.Errorf("no resource groups found for project %q", projectUUID)
	}
	return resourceGroups, nil
}

// resolvePrismElementUUIDInProject resolves a Prism Element cluster identifier to its extId using the
// project's resource groups. This is the project-scope-aware alternative to getClusters, which calls the
// global cluster-management APIs that a project-scoped user is typically not permitted to use.
// It returns ("", nil) when no Prism Element in the project's resource groups matches the identifier.
func resolvePrismElementUUIDInProject(
	ctx context.Context,
	nclient client,
	projectUUID string,
	clusterID *capxv1.NutanixResourceIdentifier,
) (string, error) {
	resourceGroups, err := listResourceGroupsForProject(ctx, nclient, projectUUID)
	if err != nil {
		return "", err
	}

	for i := range resourceGroups {
		rg := &resourceGroups[i]
		if rg.ExtId == nil || strings.TrimSpace(*rg.ExtId) == "" {
			return "", fmt.Errorf("project resource group is missing extId")
		}
		prismElements, err := nclient.ListPrismElementsByResourceGroup(ctx, *rg.ExtId)
		if err != nil {
			return "", err
		}
		if extID := findPrismElementExtID(prismElements, clusterID); extID != "" {
			return extID, nil
		}
	}

	return "", nil
}

func projectScopeAllowsCluster(
	ctx context.Context,
	nclient client,
	projectUUID string,
	clusterID *capxv1.NutanixResourceIdentifier,
) (bool, error) {
	extID, err := resolvePrismElementUUIDInProject(ctx, nclient, projectUUID, clusterID)
	if err != nil {
		return false, err
	}
	return extID != "", nil
}

func projectScopeAllowsStorageContainerInSameResourceGroup(
	ctx context.Context,
	nclient client,
	projectUUID string,
	clusterID *capxv1.NutanixResourceIdentifier,
	storageContainerName string,
) (bool, error) {
	resourceGroups, err := listResourceGroupsForProject(ctx, nclient, projectUUID)
	if err != nil {
		return false, err
	}

	for i := range resourceGroups {
		rg := &resourceGroups[i]
		if rg.ExtId == nil || strings.TrimSpace(*rg.ExtId) == "" {
			return false, fmt.Errorf("project resource group is missing extId")
		}

		prismElements, err := nclient.ListPrismElementsByResourceGroup(ctx, *rg.ExtId)
		if err != nil {
			return false, err
		}
		containers, err := nclient.ListStorageContainersByResourceGroup(ctx, *rg.ExtId)
		if err != nil {
			return false, err
		}
		if resourceGroupAllowsClusterAndStorageContainer(prismElements, containers, clusterID, storageContainerName) {
			return true, nil
		}
	}

	return false, nil
}

// findPrismElementExtID returns the extId of the Prism Element in clusters that matches clusterID
// (by UUID or name), or "" if none matches.
func findPrismElementExtID(
	clusters []converged.PrismElementInfo,
	clusterID *capxv1.NutanixResourceIdentifier,
) string {
	for _, pe := range clusters {
		if clusterID.IsUUID() && clusterID.UUID != nil &&
			strings.EqualFold(strings.TrimSpace(*clusterID.UUID), strings.TrimSpace(pe.ExtId)) {
			return strings.TrimSpace(pe.ExtId)
		}
		if clusterID.IsName() && clusterID.Name != nil &&
			strings.EqualFold(strings.TrimSpace(*clusterID.Name), strings.TrimSpace(pe.Name)) {
			return strings.TrimSpace(pe.ExtId)
		}
	}
	return ""
}

func resourceGroupAllowsStorageContainer(
	containers []converged.StorageContainerInfo,
	clusterID *capxv1.NutanixResourceIdentifier,
	storageContainerName string,
) bool {
	for _, container := range containers {
		if !strings.EqualFold(strings.TrimSpace(storageContainerName), strings.TrimSpace(container.Name)) {
			continue
		}
		if clusterID.IsUUID() && clusterID.UUID != nil &&
			strings.EqualFold(strings.TrimSpace(*clusterID.UUID), strings.TrimSpace(container.PrismElement.ExtId)) {
			return true
		}
		if clusterID.IsName() && clusterID.Name != nil &&
			strings.EqualFold(strings.TrimSpace(*clusterID.Name), strings.TrimSpace(container.PrismElement.Name)) {
			return true
		}
	}
	return false
}

func resourceGroupAllowsClusterAndStorageContainer(
	clusters []converged.PrismElementInfo,
	containers []converged.StorageContainerInfo,
	clusterID *capxv1.NutanixResourceIdentifier,
	storageContainerName string,
) bool {
	matchesCluster := func(extID, name string) bool {
		if clusterID.IsUUID() && clusterID.UUID != nil {
			return strings.EqualFold(strings.TrimSpace(*clusterID.UUID), strings.TrimSpace(extID))
		}
		if clusterID.IsName() && clusterID.Name != nil {
			return strings.EqualFold(strings.TrimSpace(*clusterID.Name), strings.TrimSpace(name))
		}
		return false
	}

	allowedClusterByExtID := map[string]struct{}{}
	for i := range clusters {
		cluster := clusters[i]
		if matchesCluster(cluster.ExtId, cluster.Name) {
			allowedClusterByExtID[strings.TrimSpace(cluster.ExtId)] = struct{}{}
		}
	}
	if len(allowedClusterByExtID) == 0 {
		return false
	}

	for i := range containers {
		container := containers[i]
		if !strings.EqualFold(strings.TrimSpace(storageContainerName), strings.TrimSpace(container.Name)) {
			continue
		}
		clusterExtID := strings.TrimSpace(container.PrismElement.ExtId)
		if _, ok := allowedClusterByExtID[clusterExtID]; ok &&
			matchesCluster(clusterExtID, container.PrismElement.Name) {
			return true
		}
	}
	return false
}
