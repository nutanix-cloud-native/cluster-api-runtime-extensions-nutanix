// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"errors"
	"fmt"
	"strings"

	clustermgmtv4ahv "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/ahv/config"
	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

// maxPhysicalGPUInstancesPerVM is the maximum number of physical (passthrough) GPU instances of the same
// profile AHV allows on a single VM. Unlike virtual GPU profiles, physical GPU profiles do not report a
// maxInstancesPerVm in the API (PhysicalGpuProfileConfiguration only carries a Name), so this platform
// limit is enforced here rather than read from Prism Central.
const maxPhysicalGPUInstancesPerVM = 8

type gpuCheck struct {
	machineDetails    *carenv1.NutanixMachineDetails
	field             string
	nclient           client
	pcVersion         string
	projectRef        *capxv1.NutanixResourceIdentifier
	failureDomainName string
	namespace         string
	kclient           ctrlclient.Client
}

func (c *gpuCheck) Name() string {
	return "NutanixGPU"
}

func (c *gpuCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}
	if len(c.machineDetails.GPUs) == 0 {
		return result
	}
	validateGPUDeviceAndProfileMix(&result, c.machineDetails.GPUs, c.field)
	if !result.Allowed {
		return result
	}
	if c.machineDetails.Cluster == nil && c.failureDomainName == "" {
		return gpuInternalError(result, c.field+".cluster", "GPU validation requires a Prism Element cluster")
	}

	peUUID, err := c.resolvePEUUID(ctx)
	if err != nil {
		if validationErr, ok := errors.AsType[*gpuValidationError](err); ok {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: validationErr.Error(),
				Field:   c.field + ".failureDomain",
			})
			return result
		}
		return gpuInternalError(
			result,
			c.field+".cluster",
			fmt.Sprintf("Failed to resolve Prism Element for GPU validation: %s. Please retry.", err),
		)
	}
	if peUUID == "" {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Prism Element %q is not available in the selected project scope.",
				c.machineDetails.Cluster,
			),
			Field: c.field + ".cluster",
		})
		return result
	}

	validator := newPEGPUValidator(c.nclient, peUUID)

	// GPU profiles that resolved successfully, kept so the per-machine composition rules can be
	// applied across the whole list once every entry has been resolved individually.
	resolvedProfiles := make([]resolvedProfileEntry, 0, len(c.machineDetails.GPUs))

	for i, gpu := range c.machineDetails.GPUs {
		field := fmt.Sprintf("%s.gpus[%d]", c.field, i)
		found, profile, err := validator.gpuExistsOnPE(ctx, gpu, c.pcVersion)
		if err != nil {
			if validationErr, ok := errors.AsType[*gpuValidationError](err); ok {
				result.Allowed = false
				result.Causes = append(result.Causes, preflight.Cause{
					Message: validationErr.Error(),
					Field:   field,
				})
				continue
			}
			return gpuInternalError(
				result,
				field,
				fmt.Sprintf(
					"Failed to validate GPU on Prism Element %q: %s. Please retry.",
					c.machineDetails.Cluster,
					err,
				),
			)
		}
		if !found {
			// A profile-typed entry refers to an AHV GPU profile, not to a GPU device.
			resourceKind := "GPU"
			if gpu.Type == capxv1.NutanixGPUIdentifierProfile {
				resourceKind = "GPU Profile"
			}
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"%s %s does not exist on Prism Element %q.",
					resourceKind,
					gpuIdentifier(gpu),
					c.machineDetails.Cluster,
				),
				Field: field,
			})
			continue
		}
		if profile != nil {
			resolvedProfiles = append(resolvedProfiles, resolvedProfileEntry{field: field, profile: *profile})
		}
	}

	validateGPUProfileComposition(&result, resolvedProfiles)

	return result
}

// validateGPUDeviceAndProfileMix rejects a gpus list that mixes device identifiers (name or
// deviceID) with GPU profiles. AHV cannot attach both on one VM, and the two identifier kinds
// are resolved against different catalogs.
func validateGPUDeviceAndProfileMix(
	result *preflight.CheckResult,
	gpus []capxv1.NutanixGPU,
	fieldPrefix string,
) {
	if result == nil || len(gpus) < 2 {
		return
	}

	firstIsDevice := gpuIsDeviceIdentifier(gpus[0].Type)
	firstIsProfile := gpuIsProfileIdentifier(gpus[0].Type)
	if !firstIsDevice && !firstIsProfile {
		return
	}

	for i := 1; i < len(gpus); i++ {
		mixed := (firstIsDevice && gpuIsProfileIdentifier(gpus[i].Type)) ||
			(firstIsProfile && gpuIsDeviceIdentifier(gpus[i].Type))
		if !mixed {
			continue
		}
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: "GPU devices identified by name or device ID cannot be mixed with GPU profiles on the same machine.",
			Field:   fmt.Sprintf("%s.gpus[%d]", fieldPrefix, i),
		})
	}
}

func gpuIsDeviceIdentifier(t capxv1.NutanixGPUIdentifierType) bool {
	return t == capxv1.NutanixGPUIdentifierName || t == capxv1.NutanixGPUIdentifierDeviceID
}

func gpuIsProfileIdentifier(t capxv1.NutanixGPUIdentifierType) bool {
	return t == capxv1.NutanixGPUIdentifierProfile
}

// resolvedProfileEntry pairs a resolved GPU profile with the field path of the gpus[] entry that
// requested it, so composition failures can be attributed to a specific entry.
type resolvedProfileEntry struct {
	field   string
	profile resolvedGPUProfile
}

// validateGPUProfileComposition enforces the two per-machine rules for GPU profiles:
//
//  1. Every profile-typed entry on a machine must resolve to the same profile, whether the profiles
//     are physical or virtual and whether they are referenced by name or by UUID. Entries are
//     compared by the resolved profile's extId, so referencing one profile by name in one entry and
//     by UUID in another is a single profile, not two.
//  2. A GPU profile may be requested at most a certain number of times on a single machine. For a
//     virtual GPU profile, that limit is its own reported maxInstancesPerVm. For a physical GPU
//     profile, which reports no such limit via the API, the limit is the fixed AHV platform maximum
//     of maxPhysicalGPUInstancesPerVM passthrough instances per VM.
//
// Both are properties of the list as a whole, so they run after every entry has been resolved.
func validateGPUProfileComposition(result *preflight.CheckResult, entries []resolvedProfileEntry) {
	if result == nil || len(entries) == 0 {
		return
	}

	first := entries[0].profile
	counts := 0
	for _, entry := range entries {
		if entry.profile.extID != first.extID {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"GPU Profiles with different types cannot be added to the same machine, "+
						"but %q and %q are both requested.",
					first.name,
					entry.profile.name,
				),
				Field: entry.field,
			})
			return
		}
		counts++
	}

	// Virtual GPU profiles self-report their per-VM limit; physical GPU profiles don't, so fall back to
	// the fixed AHV platform maximum for physical passthrough GPUs.
	maxPerVM := int64(maxPhysicalGPUInstancesPerVM)
	if first.isVirtual {
		if first.maxInstancesPerVM == nil {
			return
		}
		maxPerVM = *first.maxInstancesPerVM
	}
	if int64(counts) > maxPerVM {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"GPU Profile %q is requested %d times on a single machine, but it allows at most %d instances per machine.", //nolint:lll // Message is long.
				first.name,
				counts,
				maxPerVM,
			),
			Field: entries[len(entries)-1].field,
		})
	}
}

func (c *gpuCheck) resolvePEUUID(ctx context.Context) (string, error) {
	clusterRef := c.machineDetails.Cluster
	if c.failureDomainName != "" {
		if c.kclient == nil {
			return "", fmt.Errorf("kubernetes client is not initialized for Failure Domain %q", c.failureDomainName)
		}
		failureDomain := &capxv1.NutanixFailureDomain{}
		if err := c.kclient.Get(
			ctx,
			ctrlclient.ObjectKey{Name: c.failureDomainName, Namespace: c.namespace},
			failureDomain,
		); err != nil {
			if k8serrors.IsNotFound(err) {
				return "", &gpuValidationError{
					message: fmt.Sprintf(
						"NutanixFailureDomain %q was not found in the management cluster. Please create it and retry.",
						c.failureDomainName,
					),
				}
			}
			return "", err
		}
		clusterRef = &failureDomain.Spec.PrismElementCluster
	}
	if clusterRef == nil {
		return "", &gpuValidationError{message: "GPU validation requires a Prism Element cluster."}
	}

	scopeCtx, err := resolveProjectScopeContext(ctx, c.nclient, c.pcVersion, c.projectRef)
	if err != nil {
		return "", err
	}
	if scopeCtx != nil && scopeCtx.scope == userScopeProjectScoped {
		return resolvePrismElementUUIDInProject(ctx, c.nclient, scopeCtx.projectUUID, clusterRef)
	}
	if clusterRef.IsUUID() {
		return *clusterRef.UUID, nil
	}

	clusters, err := getClusters(ctx, c.nclient, clusterRef)
	if err != nil {
		return "", err
	}
	if len(clusters) != 1 || clusters[0].ExtId == nil {
		return "", fmt.Errorf(
			"expected exactly one Prism Element matching identifier %q, found %d",
			clusterRef,
			len(clusters),
		)
	}
	return *clusters[0].ExtId, nil
}

// peGPUValidator caches hardware API responses to prevent redundant network calls across multiple GPUs.
type peGPUValidator struct {
	nclient client
	peUUID  string

	physDeviceEntries []clustermgmtv4.PhysicalGpuProfile
	virtDeviceEntries []clustermgmtv4.VirtualGpuProfile
	devsFetched       bool

	physProfiles []clustermgmtv4ahv.PhysicalGpuProfile
	virtProfiles []clustermgmtv4ahv.VirtualGpuProfile
	profsFetched bool
}

func newPEGPUValidator(nclient client, peUUID string) *peGPUValidator {
	return &peGPUValidator{nclient: nclient, peUUID: peUUID}
}

// resolvedGPUProfile is the AHV GPU profile that a profile-typed GPU entry resolved to. Only virtual
// GPU profiles carry maxInstancesPerVm; physical GPU profiles have no such field, so their per-VM
// instance limit is the fixed maxPhysicalGPUInstancesPerVM platform maximum instead.
type resolvedGPUProfile struct {
	extID             string
	name              string
	isVirtual         bool
	maxInstancesPerVM *int64
}

// gpuExistsOnPE reports whether the GPU entry exists on the Prism Element. For a profile-typed entry
// it also returns the profile it resolved to, so that per-machine composition rules (a single vGPU
// profile per VM, within its maxInstancesPerVm) can be applied across the whole list afterwards.
func (v *peGPUValidator) gpuExistsOnPE(
	ctx context.Context,
	gpu capxv1.NutanixGPU,
	pcVersion string,
) (bool, *resolvedGPUProfile, error) {
	switch gpu.Type {
	case capxv1.NutanixGPUIdentifierName, capxv1.NutanixGPUIdentifierDeviceID:
		found, err := v.gpuDeviceExistsOnPE(ctx, gpu)
		return found, nil, err
	case capxv1.NutanixGPUIdentifierProfile:
		if !nutanixprojectpolicy.PCVersionAtLeast76(pcVersion) {
			return false, nil, &gpuValidationError{
				message: fmt.Sprintf("GPU Profile %s requires Prism Central 7.6 or later.", gpuIdentifier(gpu)),
			}
		}
		profile, err := v.gpuProfileExistsOnPE(ctx, gpu)
		if err != nil {
			return false, nil, err
		}
		return profile != nil, profile, nil
	default:
		return false, nil, fmt.Errorf("unknown GPU identifier type %q", gpu.Type)
	}
}

func (v *peGPUValidator) gpuDeviceExistsOnPE(ctx context.Context, gpu capxv1.NutanixGPU) (bool, error) {
	if !v.devsFetched {
		var err error
		if v.physDeviceEntries, err = v.nclient.ListClusterPhysicalGPUs(ctx, v.peUUID); err != nil {
			return false, err
		}
		if v.virtDeviceEntries, err = v.nclient.ListClusterVirtualGPUs(ctx, v.peUUID); err != nil {
			return false, err
		}
		v.devsFetched = true
	}

	for i := range v.physDeviceEntries {
		cfg := v.physDeviceEntries[i].PhysicalGpuConfig
		if cfg != nil && gpuMatchesDevice(gpu, cfg.DeviceName, cfg.DeviceId) {
			return true, nil
		}
	}
	for i := range v.virtDeviceEntries {
		cfg := v.virtDeviceEntries[i].VirtualGpuConfig
		if cfg != nil && gpuMatchesDevice(gpu, cfg.DeviceName, cfg.DeviceId) {
			return true, nil
		}
	}
	return false, nil
}

// gpuProfileExistsOnPE resolves a profile-typed GPU entry to exactly one AHV GPU profile on the
// Prism Element. It returns (nil, nil) when no profile matches.
func (v *peGPUValidator) gpuProfileExistsOnPE(
	ctx context.Context,
	gpu capxv1.NutanixGPU,
) (*resolvedGPUProfile, error) {
	if gpu.Profile == nil || (!gpu.Profile.IsName() && !gpu.Profile.IsUUID()) {
		return nil, &gpuValidationError{
			message: "GPU Profile requires an identifier with a name or UUID.",
		}
	}
	targetProfile := gpu.Profile

	if !v.profsFetched {
		var err error
		if v.physProfiles, err = v.nclient.ListAHVPhysicalGPUProfiles(ctx, v.peUUID); err != nil {
			return nil, err
		}
		if v.virtProfiles, err = v.nclient.ListAHVVirtualGPUProfiles(ctx, v.peUUID); err != nil {
			return nil, err
		}
		v.profsFetched = true
	}

	matches := make([]resolvedGPUProfile, 0, 1)
	for i := range v.physProfiles {
		cfg := v.physProfiles[i].Configuration
		if cfg != nil && profileMatches(targetProfile, cfg.Name, v.physProfiles[i].ExtId) {
			matches = append(matches, resolvedGPUProfile{
				extID: ptr.Deref(v.physProfiles[i].ExtId, ""),
				name:  ptr.Deref(cfg.Name, ""),
			})
		}
	}
	for i := range v.virtProfiles {
		cfg := v.virtProfiles[i].Configuration
		if cfg != nil && profileMatches(targetProfile, cfg.Name, v.virtProfiles[i].ExtId) {
			matches = append(matches, resolvedGPUProfile{
				extID:             ptr.Deref(v.virtProfiles[i].ExtId, ""),
				name:              ptr.Deref(cfg.Name, ""),
				isVirtual:         true,
				maxInstancesPerVM: cfg.MaxInstancesPerVm,
			})
		}
	}

	if len(matches) > 1 {
		return nil, &gpuValidationError{
			message: fmt.Sprintf(
				"GPU Profile %s is ambiguous on the selected Prism Element.",
				gpuIdentifier(gpu),
			),
		}
	}
	if len(matches) == 0 {
		return nil, nil
	}
	return &matches[0], nil
}

// Helpers

type gpuValidationError struct {
	message string
}

func (e *gpuValidationError) Error() string {
	return e.message
}

func matchFold(val *string, target string) bool {
	return val != nil && strings.EqualFold(strings.TrimSpace(*val), strings.TrimSpace(target))
}

func profileNameMatches(name *string, target string) bool {
	return name != nil && *name == target
}

func profileMatches(
	profile *capxv1.NutanixResourceIdentifier,
	name, uuid *string,
) bool {
	if profile == nil {
		return false
	}
	if profile.IsName() {
		return profileNameMatches(name, *profile.Name)
	}
	if profile.IsUUID() {
		return matchFold(uuid, *profile.UUID)
	}
	return false
}

func gpuMatchesDevice(gpu capxv1.NutanixGPU, deviceName *string, deviceID *int64) bool {
	switch gpu.Type {
	case capxv1.NutanixGPUIdentifierName:
		return gpu.Name != nil && matchFold(deviceName, *gpu.Name)
	case capxv1.NutanixGPUIdentifierDeviceID:
		return gpu.DeviceID != nil && deviceID != nil && *gpu.DeviceID == *deviceID
	default:
		return false
	}
}

func gpuIdentifier(gpu capxv1.NutanixGPU) string {
	switch gpu.Type {
	case capxv1.NutanixGPUIdentifierName:
		if gpu.Name != nil {
			return fmt.Sprintf("with name %q", *gpu.Name)
		}
	case capxv1.NutanixGPUIdentifierDeviceID:
		if gpu.DeviceID != nil {
			return fmt.Sprintf("with device ID %d", *gpu.DeviceID)
		}
	case capxv1.NutanixGPUIdentifierProfile:
		// The resource kind is supplied by the caller, so only the identifier is described here.
		switch {
		case gpu.Profile == nil:
		case gpu.Profile.IsName():
			return fmt.Sprintf("with name %q", *gpu.Profile.Name)
		case gpu.Profile.IsUUID():
			return fmt.Sprintf("with UUID %q", *gpu.Profile.UUID)
		}
	}
	return fmt.Sprintf("with type %q", gpu.Type)
}

func gpuInternalError(result preflight.CheckResult, field, message string) preflight.CheckResult {
	result.Allowed = false
	result.InternalError = true
	result.Causes = append(result.Causes, preflight.Cause{Message: message, Field: field})
	return result
}

func newGPUChecks(cd *checkDependencies) []preflight.Check {
	if cd == nil || cd.nclient == nil || cd.pcVersion == "" {
		return nil
	}

	checks := make([]preflight.Check, 0)

	if cd.nutanixClusterConfigSpec != nil && cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		machineDetails := &cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails
		field := controlPlaneMachineDetailsField
		if len(cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.FailureDomains) > 0 &&
			cd.cluster != nil && cd.kclient != nil {
			for _, fd := range cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.FailureDomains {
				for _, fdName := range gpuFailureDomainNames(cd, fd) {
					checks = append(checks, newFailureDomainGPUCheck(cd, machineDetails, field, fdName))
				}
			}
		} else {
			checks = append(checks, newMachineDetailsGPUCheck(machineDetails, field, cd))
		}
	}

	for mdName, workerConfig := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if workerConfig.Nutanix == nil {
			continue
		}
		machineDetails := &workerConfig.Nutanix.MachineDetails
		field := fmt.Sprintf(
			"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=='%s'].value.nutanix.machineDetails", //nolint:lll // The field is long.
			mdName,
			carenv1.WorkerConfigVariableName,
		)
		if fdName := cd.failureDomainByMachineDeploymentName[mdName]; fdName != "" &&
			cd.cluster != nil && cd.kclient != nil {
			for _, resolvedFDName := range gpuFailureDomainNames(cd, fdName) {
				checks = append(checks, newFailureDomainGPUCheck(cd, machineDetails, field, resolvedFDName))
			}
			continue
		}
		checks = append(checks, newMachineDetailsGPUCheck(machineDetails, field, cd))
	}
	return checks
}

func gpuFailureDomainNames(cd *checkDependencies, failureDomainName string) []string {
	names, err := getFailureDomainNames(cd, failureDomainName)
	if err != nil {
		return []string{failureDomainName}
	}
	return names
}

func newMachineDetailsGPUCheck(
	machineDetails *carenv1.NutanixMachineDetails,
	field string,
	cd *checkDependencies,
) *gpuCheck {
	return &gpuCheck{
		machineDetails: machineDetails,
		field:          field,
		nclient:        cd.nclient,
		pcVersion:      cd.pcVersion,
		projectRef:     machineDetails.Project,
	}
}

func newFailureDomainGPUCheck(
	cd *checkDependencies,
	machineDetails *carenv1.NutanixMachineDetails,
	field, failureDomainName string,
) *gpuCheck {
	check := newMachineDetailsGPUCheck(machineDetails, field, cd)
	check.failureDomainName = failureDomainName
	check.namespace = cd.cluster.Namespace
	check.kclient = cd.kclient
	return check
}
