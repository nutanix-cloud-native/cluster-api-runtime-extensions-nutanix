// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	projectconfig "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"

	converged "github.com/nutanix-cloud-native/prism-go-client/converged"
	v4Converged "github.com/nutanix-cloud-native/prism-go-client/converged/v4"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
)

// v3DefaultProjectName is the name of the system default project on Prism Central v3 (PC < 7.6).
const v3DefaultProjectName = "_internal"

// IsV3DefaultProjectName reports whether name is the v3 system default project.
func IsV3DefaultProjectName(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), v3DefaultProjectName)
}

var (
	// ErrProjectNotFound indicates the referenced project could not be found.
	ErrProjectNotFound = errors.New("project not found")
	// ErrProjectNotAccessible indicates the referenced project exists but is not accessible.
	ErrProjectNotAccessible = errors.New("project not accessible")
)

type resolvedProject struct {
	UUID string
	Name string
}

// PCDefaultProject holds the Prism Central default project identity.
type PCDefaultProject struct {
	UUID string
	Name string
}

func resolvePCDefaultProject(
	ctx context.Context,
	cacheParams *CacheParams,
	pcVersion string,
) (*PCDefaultProject, error) {
	pcVersion, err := prismCentralVersionForProjectResolution(ctx, cacheParams, pcVersion)
	if err != nil {
		return nil, err
	}
	if !nutanixprojectpolicy.PCVersionAtLeast76(pcVersion) {
		// v3 has no GetDefaultProject API; the system default is the project named _internal.
		return &PCDefaultProject{Name: v3DefaultProjectName}, nil
	}

	if cacheParams == nil || cacheParams.PrismManagementEndpoint == nil {
		return nil, fmt.Errorf("cache parameters or management endpoint is nil")
	}
	convergedc, err := getOrCreateConvergedClient(cacheParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get converged Prism v4 client for project resolution: %w", err)
	}

	p, err := convergedc.Projects.GetDefaultProject(ctx)
	if err != nil {
		// GetDefaultProject maps HTTP 403 and an RBAC-hidden default (HTTP 200 + empty list)
		// to ErrUnauthorized. Remap to ErrProjectNotAccessible like the other project resolvers,
		// while keeping the original error so callers that check converged.IsUnauthorized still match.
		if converged.IsUnauthorized(err) {
			return nil, fmt.Errorf("%w: default project: %w", ErrProjectNotAccessible, err)
		}
		return nil, err
	}

	rp, convErr := projectToResolvedProject(p)
	if convErr != nil {
		return nil, convErr
	}
	return &PCDefaultProject{UUID: rp.UUID, Name: rp.Name}, nil
}

func resolveProject(
	ctx context.Context,
	cacheParams *CacheParams,
	id *capxv1.NutanixResourceIdentifier,
	pcVersion string,
) (*resolvedProject, error) {
	if id == nil {
		return nil, fmt.Errorf("project identifier is nil")
	}
	pcVersion, err := prismCentralVersionForProjectResolution(ctx, cacheParams, pcVersion)
	if err != nil {
		return nil, err
	}
	if !nutanixprojectpolicy.PCVersionAtLeast76(pcVersion) {
		return resolveProjectV3(ctx, cacheParams, id)
	}

	if cacheParams == nil || cacheParams.PrismManagementEndpoint == nil {
		return nil, fmt.Errorf("cache parameters or management endpoint is nil")
	}
	convergedc, err := getOrCreateConvergedClient(cacheParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get converged Prism v4 client for project resolution: %w", err)
	}

	var project *projectconfig.Project
	switch {
	case id.IsUUID():
		project, err = convergedc.Projects.Get(ctx, strings.TrimSpace(*id.UUID))
	case id.IsName():
		project, err = resolveProjectByName(ctx, convergedc, strings.TrimSpace(*id.Name))
	default:
		return nil, fmt.Errorf("unsupported project identifier: %s", id.DisplayString())
	}
	if err != nil {
		if converged.IsNotFound(err) {
			return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, id.DisplayString())
		}
		if isAuthorizationError(err) {
			return nil, fmt.Errorf("%w: %s", ErrProjectNotAccessible, id.DisplayString())
		}
		return nil, fmt.Errorf("failed to resolve project %s: %w", id.DisplayString(), err)
	}

	return projectToResolvedProject(project)
}

func prismCentralVersionForProjectResolution(
	ctx context.Context,
	cacheParams *CacheParams,
	pcVersion string,
) (string, error) {
	if v := strings.TrimSpace(pcVersion); v != "" {
		return v, nil
	}
	if cacheParams == nil || cacheParams.PrismManagementEndpoint == nil {
		return "", fmt.Errorf("cache parameters or management endpoint is nil")
	}
	convergedc, err := getOrCreateConvergedClient(cacheParams)
	if err != nil {
		return "", fmt.Errorf("failed to get converged Prism v4 client for project resolution: %w", err)
	}
	pcVersion, err = convergedc.DomainManager.GetPrismCentralVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to determine Prism Central version for project resolution: %w", err)
	}
	return strings.TrimSpace(pcVersion), nil
}

func resolveProjectV3(
	ctx context.Context,
	cacheParams *CacheParams,
	id *capxv1.NutanixResourceIdentifier,
) (*resolvedProject, error) {
	projects, err := listV3Projects(ctx, cacheParams)
	if err != nil {
		if errors.Is(err, ErrProjectNotAccessible) {
			return nil, fmt.Errorf("%w: %s", ErrProjectNotAccessible, id.DisplayString())
		}
		return nil, err
	}
	for i := range projects {
		p := projects[i]
		switch {
		case id.IsUUID() && id.UUID != nil && strings.EqualFold(p.UUID, strings.TrimSpace(*id.UUID)):
			return &p, nil
		case id.IsName() && id.Name != nil && strings.EqualFold(p.Name, strings.TrimSpace(*id.Name)):
			return &p, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, id.DisplayString())
}

func listV3Projects(ctx context.Context, cacheParams *CacheParams) ([]resolvedProject, error) {
	v3client, err := NutanixClientCache.GetOrCreate(cacheParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get Prism Central v3 client for project resolution: %w", err)
	}
	projectResp, err := v3client.V3.ListAllProject(ctx, "")
	if err != nil {
		if isAuthorizationError(err) {
			return nil, fmt.Errorf("%w: listing projects", ErrProjectNotAccessible)
		}
		return nil, fmt.Errorf("failed to list projects via Prism Central v3 API: %w", err)
	}

	var out []resolvedProject
	for i := range projectResp.Entities {
		p := projectResp.Entities[i]
		if p == nil || p.Metadata == nil || p.Metadata.UUID == nil || p.Metadata.Name == nil {
			continue
		}
		out = append(out, resolvedProject{
			UUID: strings.TrimSpace(*p.Metadata.UUID),
			Name: strings.TrimSpace(*p.Metadata.Name),
		})
	}
	return out, nil
}

func resolveProjectByName(
	ctx context.Context,
	convergedc *v4Converged.Client,
	name string,
) (*projectconfig.Project, error) {
	escapedName := escapeODataString(name)
	projects, err := convergedc.Projects.List(ctx, converged.WithFilter(fmt.Sprintf("name eq '%s'", escapedName)))
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrProjectNotFound, name)
	}
	return &projects[0], nil
}

func projectToResolvedProject(p *projectconfig.Project) (*resolvedProject, error) {
	if p == nil {
		return nil, fmt.Errorf("nil project returned from Prism Central")
	}
	uuid := ""
	if p.ExtId != nil {
		uuid = strings.TrimSpace(*p.ExtId)
	}
	if uuid == "" {
		return nil, fmt.Errorf("project missing extId")
	}
	name := ""
	if p.Name != nil {
		name = strings.TrimSpace(*p.Name)
	}
	return &resolvedProject{
		UUID: uuid,
		Name: name,
	}, nil
}

func isAuthorizationError(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		status, ok := statusCodeFromError(e)
		if ok && (status == 401 || status == 403) {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "not authorized")
}

func statusCodeFromError(err error) (int, bool) {
	type statusProvider interface {
		GetStatus() string
	}
	if s, ok := err.(statusProvider); ok {
		return parseHTTPStatusCode(s.GetStatus())
	}

	v := reflectValue(err)
	if !v.IsValid() {
		return 0, false
	}
	statusField := v.FieldByName("Status")
	if !statusField.IsValid() || statusField.Kind() != reflect.String {
		return 0, false
	}
	return parseHTTPStatusCode(statusField.String())
}

func parseHTTPStatusCode(status string) (int, bool) {
	if status == "" {
		return 0, false
	}
	code, err := strconv.Atoi(strings.SplitN(status, " ", 2)[0])
	if err != nil {
		return 0, false
	}
	return code, true
}

func reflectValue(err error) reflect.Value {
	v := reflect.ValueOf(err)
	if !v.IsValid() {
		return reflect.Value{}
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	return v
}

// ProjectMatchesDefault returns true when id refers to the same project as def (by UUID or name).
func ProjectMatchesDefault(id *capxv1.NutanixResourceIdentifier, def *PCDefaultProject) bool {
	if id == nil || def == nil {
		return false
	}
	switch {
	case id.IsUUID() && id.UUID != nil && strings.EqualFold(strings.TrimSpace(*id.UUID), def.UUID):
		return true
	case id.IsName() && id.Name != nil && def.Name != "" && strings.EqualFold(strings.TrimSpace(*id.Name), def.Name):
		return true
	default:
		return false
	}
}
