// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"slices"
	"strings"

	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	metroFailureDomainPrefix     = "NutanixMetro/"
	metroSiteFailureDomainPrefix = "NutanixMetroSite/"
)

type failureDomainCheck struct {
	failureDomainName string
	namespace         string
	field             string
	kclient           ctrlclient.Client
	nclient           client

	// The error message set if error hit when adding the check
	errMessage   *string
	pcVersion    string
	projectRef   *capxv1.NutanixResourceIdentifier
	podCIDRs     []string
	serviceCIDRs []string
}

func clusterNetworkPodCIDRs(cd *checkDependencies) []string {
	if cd == nil || cd.cluster == nil {
		return nil
	}
	return cd.cluster.Spec.ClusterNetwork.Pods.CIDRBlocks
}

func clusterNetworkServiceCIDRs(cd *checkDependencies) []string {
	if cd == nil || cd.cluster == nil {
		return nil
	}
	return cd.cluster.Spec.ClusterNetwork.Services.CIDRBlocks
}

func (fdc *failureDomainCheck) Name() string {
	return "NutanixFailureDomain"
}

func newFailureDomainChecks(cd *checkDependencies) []preflight.Check {
	checks := []preflight.Check{}

	if cd == nil || cd.kclient == nil || cd.nclient == nil || cd.pcVersion == "" {
		return checks
	}

	// For the failure domains configured for control-plane nodes
	if cd.nutanixClusterConfigSpec != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		for _, fd := range cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.FailureDomains {
			if fd != "" {
				fdNames, err := getFailureDomainNames(cd, fd)
				if err != nil {
					// log the error and continue
					cd.log.Error(err, fmt.Sprintf("set the errMessage for failureDomain %s due to error", fd))
					check := &failureDomainCheck{
						failureDomainName: fd,
						namespace:         cd.cluster.Namespace,
						field:             cpFailureDomainsField,
						kclient:           cd.kclient,
						nclient:           cd.nclient,
						pcVersion:         cd.pcVersion,
						projectRef:        cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails.Project,
						podCIDRs:          clusterNetworkPodCIDRs(cd),
						serviceCIDRs:      clusterNetworkServiceCIDRs(cd),
					}
					check.errMessage = new(err.Error())

					checks = append(checks, check)
					continue
				}

				for _, fdName := range fdNames {
					checks = append(checks, &failureDomainCheck{
						failureDomainName: fdName,
						namespace:         cd.cluster.Namespace,
						field:             cpFailureDomainsField,
						kclient:           cd.kclient,
						nclient:           cd.nclient,
						pcVersion:         cd.pcVersion,
						projectRef:        cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails.Project,
						podCIDRs:          clusterNetworkPodCIDRs(cd),
						serviceCIDRs:      clusterNetworkServiceCIDRs(cd),
					})
				}
			}
		}
	}

	// For the failure domains configured for worker nodes
	if cd.cluster != nil &&
		cd.cluster.Spec.Topology.IsDefined() &&
		len(cd.cluster.Spec.Topology.Workers.MachineDeployments) > 0 {
		for i := range cd.cluster.Spec.Topology.Workers.MachineDeployments {
			md := &cd.cluster.Spec.Topology.Workers.MachineDeployments[i]
			if md.FailureDomain == "" {
				continue
			}
			fdNames, err := getFailureDomainNames(cd, md.FailureDomain)
			if err != nil {
				// log the error and continue
				cd.log.Error(
					err,
					fmt.Sprintf("set the checkResult for failureDomain %s due to error", md.FailureDomain),
				)
				check := &failureDomainCheck{
					failureDomainName: md.FailureDomain,
					namespace:         cd.cluster.Namespace,
					field:             cpFailureDomainsField,
					kclient:           cd.kclient,
					nclient:           cd.nclient,
					podCIDRs:          clusterNetworkPodCIDRs(cd),
					serviceCIDRs:      clusterNetworkServiceCIDRs(cd),
				}
				check.errMessage = new(err.Error())

				checks = append(checks, check)
				continue
			}

			for _, fdName := range fdNames {
				checks = append(checks, &failureDomainCheck{
					failureDomainName: fdName,
					namespace:         cd.cluster.Namespace,
					field: fmt.Sprintf(
						"$.spec.topology.workers.machineDeployments[?@.name==%q].failureDomain",
						md.Name,
					),
					kclient:   cd.kclient,
					nclient:   cd.nclient,
					pcVersion: cd.pcVersion,
					projectRef: func() *capxv1.NutanixResourceIdentifier {
						workerSpec, ok := cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName[md.Name]
						if !ok || workerSpec == nil || workerSpec.Nutanix == nil {
							return nil
						}
						return workerSpec.Nutanix.MachineDetails.Project
					}(),
					podCIDRs:     clusterNetworkPodCIDRs(cd),
					serviceCIDRs: clusterNetworkServiceCIDRs(cd),
				})
			}
		}
	}

	return checks
}

func (fdc *failureDomainCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: true,
	}

	if fdc.errMessage != nil {
		// return the check result with errMessage as Cause
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to check failureDomain %q: %s",
				fdc.failureDomainName,
				*fdc.errMessage,
			),
			Field: fdc.field,
		})
		return result
	}

	// Fetch the referent failure domain object
	fdObj := &capxv1.NutanixFailureDomain{}
	fdKey := ctrlclient.ObjectKey{Name: fdc.failureDomainName, Namespace: fdc.namespace}
	if err := fdc.kclient.Get(ctx, fdKey, fdObj); err != nil {
		if errors.IsNotFound(err) {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"NutanixFailureDomain %q was not found in the management cluster. Please create it and retry.", //nolint:lll // Message is long.
					fdc.failureDomainName,
				),
				Field: fdc.field,
			})
			return result
		}

		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to get NutanixFailureDomain %q: %s This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
				fdc.failureDomainName,
				err,
			),
			Field: fdc.field,
		})
		return result
	}

	// Validate the failure domain configuration
	// Validate spec.prismElementCluster configuration
	peIdentifier := fdObj.Spec.PrismElementCluster
	scopeCtx, err := resolveProjectScopeContext(ctx, fdc.nclient, fdc.pcVersion, fdc.projectRef)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to resolve project scope for Failure Domain %q: %s. Please retry.",
				fdc.failureDomainName,
				err,
			),
			Field: fdc.field,
		})
		return result
	}

	var peUUID string
	if scopeCtx != nil && scopeCtx.scope == userScopeProjectScoped {
		// Project-scoped users typically cannot call the global cluster-management APIs used by
		// getClusters, so resolve the Prism Element via the project's resource groups. This also
		// validates that the Prism Element is available within the project scope.
		peUUID, err = resolvePrismElementUUIDInProject(ctx, fdc.nclient, scopeCtx.projectUUID, &peIdentifier)
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to resolve project resource-group capabilities for Failure Domain %q: %s. Please retry.",
					fdc.failureDomainName,
					err,
				),
				Field: fdc.field,
			})
			return result
		}
		if peUUID == "" {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Prism Element cluster %q referenced by Failure Domain %q is not available in selected project scope.",
					peIdentifier,
					fdc.failureDomainName,
				),
				Field: fdc.field,
			})
			return result
		}
	} else {
		peClusters, err := getClusters(ctx, fdc.nclient, &peIdentifier)
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to check if the Prism Element cluster %q, referenced by Failure Domain %q, exists: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
					peIdentifier,
					fdc.failureDomainName,
					err,
				),
				Field: fdc.field,
			})
			return result
		}
		if len(peClusters) != 1 || peClusters[0].ExtId == nil {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Found %d Prism Element cluster(s) that match identifier %q. There must be exactly 1 Cluster that matches this identifier.", //nolint:lll // Message is long.
					len(peClusters),
					peIdentifier,
				),
				Field: fdc.field,
			})
			return result
		}
		peUUID = *peClusters[0].ExtId
	}

	// Parsed once outside the loop below: the same Pod/Service CIDRs apply to every subnet in this
	// Failure Domain. Parse errors are ignored here because invalid Pod/Service CIDR configuration
	// is already reported by NutanixCIDRValidation; this check only needs the subnets it can parse.
	podCIDRs, _ := parseCIDRBlocks(fdc.podCIDRs)
	serviceCIDRs, _ := parseCIDRBlocks(fdc.serviceCIDRs)

	// Validate spec.subnets configuration
	for _, id := range fdObj.Spec.Subnets {
		subnets, err := getSubnets(ctx, fdc.nclient, &id)
		if scopeCtx != nil {
			subnets, err = getSubnetsInProject(ctx, fdc.nclient, &id, scopeCtx.projectUUID)
		}
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to get subnet %q referenced by the Failure Domain %q: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
					id,
					fdc.failureDomainName,
					err,
				),
				Field: fdc.field,
			})
			continue
		}

		// Filter to subnets reachable from this PE (overlay by SubnetType, else ClusterReference /
		// ClusterReferenceList), matching CAPX GetSubnetUUID.
		filteredSubnets := filterSubnetsForPrismElement(subnets, peUUID)
		// Prefer subnets owned by the project over those merely shared with it, matching the VM Image and
		// VM Profile checks. Applied only as a tie-breaker after Prism Element narrowing, so a valid shared
		// subnet on the referenced Prism Element is never discarded in favour of an owned subnet on a
		// different Prism Element.
		if len(filteredSubnets) > 1 && scopeCtx != nil {
			filteredSubnets = preferProjectOwnedSubnets(filteredSubnets, scopeCtx.projectUUID)
		}

		if len(filteredSubnets) != 1 {
			result.Allowed = false
			// Mirror the NutanixSubnet check's guidance. The Prism Element cluster is referenced by the
			// Failure Domain, but the project is referenced by the cluster (machineDetails.project), not by
			// the Failure Domain. When the check ran project-scoped, the subnet lookup is filtered by project
			// ownership/sharing, so a project-scope mismatch is one of the reasons zero subnets are found;
			// call that out explicitly, attributed correctly.
			var message string
			if scopeCtx != nil {
				message = fmt.Sprintf(
					"Found %d Subnets in Prism Central that match identifier %q. There must be exactly 1 Subnet that is attached to the Prism Element cluster %q (referenced by Failure Domain %q) and is owned by or shared with the referenced Prism Central project %q. Check that the subnet exists and satisfies both conditions, then retry.", //nolint:lll // Message is long.
					len(filteredSubnets),
					id,
					peIdentifier,
					fdc.failureDomainName,
					fdc.projectRef,
				)
			} else {
				message = fmt.Sprintf(
					"Found %d Subnets in Prism Central that match identifier %q. There must be exactly 1 Subnet that is attached to the Prism Element cluster %q (referenced by Failure Domain %q). Check that the subnet exists and is attached to that Prism Element cluster, then retry.", //nolint:lll // Message is long.
					len(filteredSubnets),
					id,
					peIdentifier,
					fdc.failureDomainName,
				)
			}
			result.Causes = append(result.Causes, preflight.Cause{
				Message: message,
				Field:   fdc.field,
			})
			continue
		}

		prefixes, err := extractIPv4PrefixesFromSubnet(&filteredSubnets[0])
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to extract IPv4 CIDR for subnet %q referenced by Failure Domain %q: %s. Please retry.",
					id,
					fdc.failureDomainName,
					err,
				),
				Field: fdc.field,
			})
			continue
		}
		if len(prefixes) == 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Subnet %q referenced by Failure Domain %q appears to use external IPAM (no IP configuration found). "+
					"CIDR overlap validation will be skipped for this subnet.",
				id,
				fdc.failureDomainName,
			))
			continue
		}

		resolvedSubnets := make([]resolvedNodeSubnet, 0, len(prefixes))
		for _, prefix := range prefixes {
			resolvedSubnets = append(resolvedSubnets, resolvedNodeSubnet{
				prefix: prefix,
				field:  fdc.field,
				name:   subnetIdentifierForMessage(id),
			})
		}
		validateSubnetAndPodCIDRsNotOverlapping(&result, podCIDRs, resolvedSubnets)
		validateSubnetAndServiceCIDRsNotOverlapping(&result, serviceCIDRs, resolvedSubnets)
	}

	return result
}

// filterSubnetsForPrismElement returns subnets that belong to the Prism Element identified by peUUID.
func filterSubnetsForPrismElement(subnets []netv4.Subnet, peUUID string) []netv4.Subnet {
	filtered := make([]netv4.Subnet, 0, len(subnets))
	for i := range subnets {
		if subnetBelongsToPrismElement(&subnets[i], peUUID) {
			filtered = append(filtered, subnets[i])
		}
	}
	return filtered
}

// subnetBelongsToPrismElement checks if a subnet belongs to the specified Prism Element, matching CAPX
// GetSubnetUUID / subnetBelongsToCluster:
//   - Overlay subnets (SubnetType OVERLAY) are reachable from any PE.
//   - Otherwise ClusterReference and/or ClusterReferenceList must name the PE.
//
// Non-overlay subnets may have:
//   - Both ClusterReference and ClusterReferenceList present (most cases; ClusterReference is being deprecated)
//   - Only ClusterReference present: older AOS (<=7.0)
//   - Only ClusterReferenceList present: PC-based vSwitch subnets (>=7.3)
func subnetBelongsToPrismElement(subnet *netv4.Subnet, peUUID string) bool {
	if subnet.SubnetType != nil && *subnet.SubnetType == netv4.SUBNETTYPE_OVERLAY {
		return true
	}
	if subnet.ClusterReference != nil && *subnet.ClusterReference == peUUID {
		return true
	}
	if subnet.ClusterReferenceList != nil && slices.Contains(subnet.ClusterReferenceList, peUUID) {
		return true
	}
	return false
}

func getSubnetsInProject(
	ctx context.Context,
	client client,
	subnetID *capxv1.NutanixResourceIdentifier,
	projectUUID string,
) ([]netv4.Subnet, error) {
	projectUUID = escapeODataString(projectUUID)
	switch {
	case subnetID.IsUUID():
		filter := fmt.Sprintf(
			"extId eq '%s' and (projectExtId eq '%s' or sharedWithProjects/any(a:a eq '%s'))",
			escapeODataString(*subnetID.UUID),
			projectUUID,
			projectUUID,
		)
		return client.ListSubnetsByFilter(ctx, filter)
	case subnetID.IsName():
		filter := fmt.Sprintf(
			"name eq '%s' and (projectExtId eq '%s' or sharedWithProjects/any(a:a eq '%s'))",
			escapeODataString(*subnetID.Name),
			projectUUID,
			projectUUID,
		)
		return client.ListSubnetsByFilter(ctx, filter)
	default:
		return nil, fmt.Errorf("subnet identifier is missing both name and uuid, identifier type: %s", subnetID.Type)
	}
}

// getSubnets returns the subnets found in PC with the input identifier.
func getSubnets(
	ctx context.Context,
	client client,
	subnetId *capxv1.NutanixResourceIdentifier,
) (
	[]netv4.Subnet,
	error,
) {
	switch {
	case subnetId.IsUUID():
		resp, err := client.GetSubnetById(ctx, subnetId.UUID)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			// No subnet returned.
			return []netv4.Subnet{}, nil
		}
		subnet, err := subnetFromGetSubnetData(resp.GetData())
		if err != nil {
			return nil, fmt.Errorf("failed to get data returned by GetSubnetById")
		}
		return []netv4.Subnet{subnet}, nil
	case subnetId.IsName():
		filter_ := fmt.Sprintf("name eq '%s'", *subnetId.Name)
		return client.ListSubnetsByFilter(ctx, filter_)
	default:
		return nil, fmt.Errorf("subnet identifier is missing both name and uuid, identifier type: %s", subnetId.Type)
	}
}

func subnetFromGetSubnetData(data any) (netv4.Subnet, error) {
	switch subnet := data.(type) {
	case netv4.Subnet:
		return subnet, nil
	case *netv4.Subnet:
		if subnet == nil {
			return netv4.Subnet{}, fmt.Errorf("subnet data is nil")
		}
		return *subnet, nil
	default:
		return netv4.Subnet{}, fmt.Errorf("unexpected subnet data type %T", data)
	}
}

func isNutanixMetroFailureDomain(fdName string) bool {
	return strings.HasPrefix(fdName, metroFailureDomainPrefix)
}

func isNutanixMetroSiteFailureDomain(fdName string) bool {
	return strings.HasPrefix(fdName, metroSiteFailureDomainPrefix)
}

func getFailureDomainNames(cd *checkDependencies, fd string) ([]string, error) {
	fdNames := []string{}
	namespace := cd.cluster.Namespace
	ctx := context.TODO()

	switch {
	case isNutanixMetroFailureDomain(fd):
		// for NutanixMetro
		metroName := fd[len(metroFailureDomainPrefix):]
		cd.log.Info("handling NutanixMetro", "metroName", metroName)
		metroObj := &capxv1.NutanixMetro{}
		metroKey := ctrlclient.ObjectKey{Name: metroName, Namespace: namespace}
		if err := cd.kclient.Get(ctx, metroKey, metroObj); err != nil {
			return nil, fmt.Errorf(
				"failed to fetch the NutanixMetro %s referenced by failureDomain %s: %w",
				metroName,
				fd,
				err,
			)
		}
		for _, fdRef := range metroObj.Spec.FailureDomains {
			if fdRef.Name != "" {
				fdNames = append(fdNames, fdRef.Name)
			}
		}

	case isNutanixMetroSiteFailureDomain(fd):
		// for NutanixMetroSite
		metroSiteName := fd[len(metroSiteFailureDomainPrefix):]
		cd.log.Info("handling NutanixMetroSite", "metroSiteName", metroSiteName)
		metroSiteObj := &capxv1.NutanixMetroSite{}
		metroSiteKey := ctrlclient.ObjectKey{Name: metroSiteName, Namespace: namespace}
		if err := cd.kclient.Get(ctx, metroSiteKey, metroSiteObj); err != nil {
			return nil, fmt.Errorf(
				"failed to fetch the NutanixMetroSite %s referenced by failureDomain %s: %w",
				metroSiteName,
				fd,
				err,
			)
		}

		// fetch NutanixMetro referenced by the NutanixMetroSite
		metroName := metroSiteObj.Spec.MetroRef.Name
		metroObj := &capxv1.NutanixMetro{}
		metroKey := ctrlclient.ObjectKey{Name: metroName, Namespace: namespace}
		if err := cd.kclient.Get(ctx, metroKey, metroObj); err != nil {
			return nil, fmt.Errorf(
				"failed to fetch the NutanixMetro %s referenced by NutanixMetroSite %s, failureDomain %s: %w",
				metroName,
				metroSiteName,
				fd,
				err,
			)
		}

		foundPreferredFD := false
		for _, fdRef := range metroObj.Spec.FailureDomains {
			if fdRef.Name != "" {
				fdNames = append(fdNames, fdRef.Name)
				if fdRef.Name == metroSiteObj.Spec.PreferredFailureDomain.Name {
					foundPreferredFD = true
				}
			}
		}
		if !foundPreferredFD {
			return nil, fmt.Errorf(
				"the NutanixMetroSite %s preferredFailureDomain %s is not from the referenced NutanixMetro %s failureDomains %v",
				metroSiteName,
				metroSiteObj.Spec.PreferredFailureDomain.Name,
				metroName,
				fdNames,
			)
		}

	default:
		// should be NuanixFailureDomain name
		fdNames = append(fdNames, fd)
	}

	return fdNames, nil
}
