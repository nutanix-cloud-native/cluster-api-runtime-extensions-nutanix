// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"net/netip"

	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	clusterNetworkPodsFieldPath     = "$.spec.clusterNetwork.pods.cidrBlocks"
	clusterNetworkServicesFieldPath = "$.spec.clusterNetwork.services.cidrBlocks"
)

// nutanixSubnet pairs a Nutanix subnet identifier with its JSONPath field and the PE cluster it belongs to.
type nutanixSubnet struct {
	id      capxv1.NutanixResourceIdentifier
	field   string
	cluster *capxv1.NutanixResourceIdentifier
}

// resolvedNodeSubnet holds a resolved IPv4 prefix for a node subnet along with its field path and display name.
type resolvedNodeSubnet struct {
	prefix netip.Prefix
	field  string
	name   string
}

// cidrValidationCheck validates Pod/Service CIDRs for size, mutual overlap, and overlap with node subnets.
type cidrValidationCheck struct {
	cd *checkDependencies

	resolveNodeSubnetsFunc func(
		ctx context.Context,
		nclient client,
		subnets []nutanixSubnet,
	) ([]resolvedNodeSubnet, []string, error)
}

func (c *cidrValidationCheck) Name() string {
	return "NutanixCIDRValidation"
}

func (c *cidrValidationCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: true,
	}

	if c == nil || c.cd == nil || c.cd.cluster == nil {
		return result
	}

	var podCIDRBlocks []string
	var serviceCIDRBlocks []string
	cn := c.cd.cluster.Spec.ClusterNetwork
	podCIDRBlocks = cn.Pods.CIDRBlocks
	serviceCIDRBlocks = cn.Services.CIDRBlocks

	podCIDRs, err := parseCIDRBlocks(podCIDRBlocks)
	if err != nil {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("Invalid Pod CIDR configuration: %s", err),
			Field:   clusterNetworkPodsFieldPath,
		})
	}

	serviceCIDRs, err := parseCIDRBlocks(serviceCIDRBlocks)
	if err != nil {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("Invalid Service CIDR configuration: %s", err),
			Field:   clusterNetworkServicesFieldPath,
		})
	}

	validatePodCIDR(&result, podCIDRs)
	validateServiceCIDR(&result, serviceCIDRs)
	validatePodAndServiceCIDRsNotOverlapping(&result, podCIDRs, serviceCIDRs)

	nutanixSubnets := collectNutanixSubnets(c.cd)
	if len(nutanixSubnets) == 0 {
		return result
	}

	if c.cd.nclient == nil {
		return result
	}

	resolvedSubnets, resolveWarnings, err := c.resolveNodeSubnetsFunc(ctx, c.cd.nclient, nutanixSubnets)
	result.Warnings = append(result.Warnings, resolveWarnings...)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to resolve node subnet CIDRs from Prism Central: %s. This is usually a temporary error. Please retry.",
				err,
			),
		})
		return result
	}

	validateSubnetAndPodCIDRsNotOverlapping(&result, podCIDRs, resolvedSubnets)
	validateSubnetAndServiceCIDRsNotOverlapping(&result, serviceCIDRs, resolvedSubnets)

	return result
}

// newCIDRValidationChecks creates the CIDR validation preflight check.
func newCIDRValidationChecks(cd *checkDependencies) []preflight.Check {
	if cd == nil || cd.cluster == nil {
		return nil
	}

	return []preflight.Check{
		&cidrValidationCheck{
			cd:                     cd,
			resolveNodeSubnetsFunc: resolveNodeSubnets,
		},
	}
}

// parseCIDRBlocks parses a slice of CIDR strings into masked netip.Prefix values.
func parseCIDRBlocks(cidrs []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CIDR %q: %w", cidr, err)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

// validatePodCIDR checks that Pod CIDRs are large enough for multi-node clusters.
func validatePodCIDR(result *preflight.CheckResult, prefixes []netip.Prefix) {
	if result == nil {
		return
	}

	for _, prefix := range prefixes {
		maskSize := prefix.Bits()
		maxNodes := maxNodesForPodCIDR(maskSize, 24)

		switch {
		case maskSize >= 24:
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Pod CIDR %q has prefix /%d, which is too small for multi-node clusters. "+
						"With a /24 node mask, this supports at most %d node(s). Use a larger Pod CIDR (for example /16).",
					prefix.String(),
					maskSize,
					maxNodes,
				),
				Field: clusterNetworkPodsFieldPath,
			})
		case maskSize >= 21:
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Pod CIDR %q has prefix /%d, which supports only %d node(s) with a /24 node mask. "+
					"Consider a larger Pod CIDR (for example /16).",
				prefix.String(),
				maskSize,
				maxNodes,
			))
		}
	}
}

// validateServiceCIDR checks that Service CIDRs are large enough for scaling.
func validateServiceCIDR(result *preflight.CheckResult, prefixes []netip.Prefix) {
	if result == nil {
		return
	}

	for _, prefix := range prefixes {
		maskSize := prefix.Bits()
		totalIPs, hasCapacity := ipv4CIDRCapacity(prefix)

		switch {
		case maskSize >= 24:
			result.Allowed = false
			message := fmt.Sprintf(
				"Service CIDR %q is too small with prefix /%d (/24 or smaller). Minimum /23 recommended.",
				prefix.String(),
				maskSize,
			)
			if hasCapacity {
				message = fmt.Sprintf(
					"%s This CIDR supports up to %d service IPs.",
					message,
					totalIPs,
				)
			}
			result.Causes = append(result.Causes, preflight.Cause{
				Message: message,
				Field:   clusterNetworkServicesFieldPath,
			})
		case maskSize > 20:
			if hasCapacity {
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"Service CIDR %q is tight with prefix /%d and supports up to %d service IPs. "+
						"You may encounter scaling issues if service count grows significantly.",
					prefix.String(),
					maskSize,
					totalIPs,
				))
				continue
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Service CIDR %q is tight with prefix /%d. You may encounter scaling issues if service count grows significantly.",
				prefix.String(),
				maskSize,
			))
		}
	}
}

// validatePodAndServiceCIDRsNotOverlapping checks that Pod and Service CIDRs do not overlap.
func validatePodAndServiceCIDRsNotOverlapping(
	result *preflight.CheckResult,
	podCIDRs, serviceCIDRs []netip.Prefix,
) {
	if result == nil {
		return
	}

	for _, podCIDR := range podCIDRs {
		for _, serviceCIDR := range serviceCIDRs {
			if prefixesOverlap(podCIDR, serviceCIDR) {
				result.Allowed = false
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf(
						"Pod CIDR %q overlaps with Service CIDR %q. Use non-overlapping Pod and Service CIDR ranges and retry.",
						podCIDR.String(),
						serviceCIDR.String(),
					),
					Field: clusterNetworkPodsFieldPath,
				})
			}
		}
	}
}

// validateSubnetAndPodCIDRsNotOverlapping checks that no node subnet overlaps with any Pod CIDR.
func validateSubnetAndPodCIDRsNotOverlapping(
	result *preflight.CheckResult,
	podCIDRs []netip.Prefix,
	subnets []resolvedNodeSubnet,
) {
	if result == nil {
		return
	}

	for _, podCIDR := range podCIDRs {
		for _, nodeSubnet := range subnets {
			if prefixesOverlap(podCIDR, nodeSubnet.prefix) {
				result.Allowed = false
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf(
						"Pod CIDR %q overlaps with node subnet %q (CIDR %q). Use non-overlapping ranges and retry.",
						podCIDR.String(),
						nodeSubnet.name,
						nodeSubnet.prefix.String(),
					),
					Field: nodeSubnet.field,
				})
			}
		}
	}
}

// validateSubnetAndServiceCIDRsNotOverlapping checks that no node subnet overlaps with any Service CIDR.
func validateSubnetAndServiceCIDRsNotOverlapping(
	result *preflight.CheckResult,
	serviceCIDRs []netip.Prefix,
	subnets []resolvedNodeSubnet,
) {
	if result == nil {
		return
	}

	for _, serviceCIDR := range serviceCIDRs {
		for _, nodeSubnet := range subnets {
			if prefixesOverlap(serviceCIDR, nodeSubnet.prefix) {
				result.Allowed = false
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf(
						"Service CIDR %q overlaps with node subnet %q (CIDR %q). Use non-overlapping ranges and retry.",
						serviceCIDR.String(),
						nodeSubnet.name,
						nodeSubnet.prefix.String(),
					),
					Field: nodeSubnet.field,
				})
			}
		}
	}
}

// ipv4CIDRCapacity returns the total number of IP addresses in an IPv4 prefix.
func ipv4CIDRCapacity(prefix netip.Prefix) (int, bool) {
	if !prefix.Addr().Is4() {
		return 0, false
	}
	hostBits := 32 - prefix.Bits()
	if hostBits < 0 || hostBits > 31 {
		// hostBits==32 would overflow int32-sized shifts in some environments.
		return 0, false
	}
	return 1 << hostBits, true
}

// maxNodesForPodCIDR calculates how many nodes a Pod CIDR can support given a per-node mask size.
func maxNodesForPodCIDR(podMaskSize, nodeMaskSize int) int {
	if podMaskSize >= nodeMaskSize {
		return 1
	}
	return 1 << (nodeMaskSize - podMaskSize)
}

// prefixesOverlap returns true if two IP prefixes share any addresses.
func prefixesOverlap(a, b netip.Prefix) bool {
	return a.Overlaps(b)
}

// collectNutanixSubnets gathers subnet identifiers from control plane and worker machine deployment
// configs, paired with their field paths and PE cluster references.
func collectNutanixSubnets(cd *checkDependencies) []nutanixSubnet {
	if cd == nil {
		return nil
	}

	subnets := make([]nutanixSubnet, 0)

	if cd.nutanixClusterConfigSpec != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		cpDetails := &cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails
		for i, subnetID := range cpDetails.Subnets {
			subnets = append(subnets, nutanixSubnet{
				id: subnetID,
				field: fmt.Sprintf(
					"$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.machineDetails.subnets[%d]", //nolint:lll // Field path is long.
					i,
				),
				cluster: cpDetails.Cluster,
			})
		}
	}

	for mdName, worker := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if worker == nil || worker.Nutanix == nil {
			continue
		}
		workerDetails := &worker.Nutanix.MachineDetails
		for i, subnetID := range workerDetails.Subnets {
			subnets = append(subnets, nutanixSubnet{
				id: subnetID,
				field: fmt.Sprintf(
					"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=workerConfig].value.nutanix.machineDetails.subnets[%d]", //nolint:lll // Field path is long.
					mdName,
					i,
				),
				cluster: workerDetails.Cluster,
			})
		}
	}

	return subnets
}

// subnetIdentifierKey returns a string key for caching resolved prefixes by subnet identifier type and value.
func subnetIdentifierKey(id capxv1.NutanixResourceIdentifier) string {
	switch {
	case id.IsUUID():
		return "uuid:" + *id.UUID
	case id.IsName():
		return "name:" + *id.Name
	default:
		return ""
	}
}

// resolveNodeSubnets fetches subnet details from Prism Central and extracts IPv4 prefixes,
// with caching to avoid duplicate API calls.
func resolveNodeSubnets(
	ctx context.Context,
	nclient client,
	subnets []nutanixSubnet,
) ([]resolvedNodeSubnet, []string, error) {
	resolved := make([]resolvedNodeSubnet, 0)
	var warnings []string
	// Cache resolved prefixes by subnet key to avoid duplicate API calls
	// when the same subnet is referenced by multiple node pools.
	prefixCache := map[string][]netip.Prefix{}

	for _, subnet := range subnets {
		cacheKey := subnetIdentifierKey(subnet.id)

		cached, ok := prefixCache[cacheKey]
		if !ok {
			pcSubnets, err := getSubnets(ctx, nclient, &subnet.id)
			if err != nil {
				return nil, warnings, fmt.Errorf(
					"failed to get subnet %q: %w", subnetIdentifierForMessage(subnet.id), err,
				)
			}
			if len(pcSubnets) == 0 {
				return nil, warnings, fmt.Errorf(
					"found no subnets for identifier %q", subnetIdentifierForMessage(subnet.id),
				)
			}

			if subnet.id.IsName() && len(pcSubnets) > 1 {
				pcSubnets = filterSubnetsByPECluster(ctx, nclient, pcSubnets, subnet.cluster)
				if len(pcSubnets) != 1 {
					return nil, warnings, fmt.Errorf(
						"found %d subnets matching name %q on the target Prism Element cluster; "+
							"there must be exactly 1; use subnet UUID instead",
						len(pcSubnets), *subnet.id.Name,
					)
				}
			}

			prefixes := make([]netip.Prefix, 0)
			for i := range pcSubnets {
				subnetPrefixes, err := extractIPv4PrefixesFromSubnet(&pcSubnets[i])
				if err != nil {
					return nil, warnings, fmt.Errorf(
						"failed to extract IPv4 CIDR for subnet %q: %w",
						subnetIdentifierForMessage(subnet.id),
						err,
					)
				}
				prefixes = append(prefixes, subnetPrefixes...)
			}

			if len(prefixes) == 0 {
				warnings = append(warnings, fmt.Sprintf(
					"Subnet %q appears to use external IPAM (no IP configuration found). "+
						"CIDR overlap validation will be skipped for this subnet.",
					subnetIdentifierForMessage(subnet.id),
				))
			}

			prefixCache[cacheKey] = prefixes
			cached = prefixes
		}

		for _, prefix := range cached {
			resolved = append(resolved, resolvedNodeSubnet{
				prefix: prefix,
				field:  subnet.field,
				name:   subnetIdentifierForMessage(subnet.id),
			})
		}
	}

	return resolved, warnings, nil
}

// filterSubnetsByPECluster narrows a set of subnets to those belonging to the target PE cluster
// or to no PE (overlay subnets).
func filterSubnetsByPECluster(
	ctx context.Context,
	nclient client,
	subnets []netv4.Subnet,
	clusterID *capxv1.NutanixResourceIdentifier,
) []netv4.Subnet {
	if clusterID == nil {
		return subnets
	}

	peUUID, err := resolvePEClusterUUID(ctx, nclient, clusterID)
	if err != nil || peUUID == "" {
		return subnets
	}

	filtered := make([]netv4.Subnet, 0)
	for i := range subnets {
		if subnets[i].ClusterReference == nil || *subnets[i].ClusterReference == peUUID {
			filtered = append(filtered, subnets[i])
		}
	}
	return filtered
}

// resolvePEClusterUUID resolves a PE cluster identifier to its UUID, looking it up by name if necessary.
func resolvePEClusterUUID(
	ctx context.Context,
	nclient client,
	clusterID *capxv1.NutanixResourceIdentifier,
) (string, error) {
	if clusterID.IsUUID() {
		return *clusterID.UUID, nil
	}
	clusters, err := getClusters(ctx, nclient, clusterID)
	if err != nil {
		return "", fmt.Errorf(
			"failed to resolve PE cluster %q: %w",
			subnetIdentifierForMessage(*clusterID), err,
		)
	}
	if len(clusters) != 1 {
		return "", fmt.Errorf(
			"found %d PE clusters matching %q; expected exactly 1",
			len(clusters), subnetIdentifierForMessage(*clusterID),
		)
	}
	return *clusters[0].ExtId, nil
}

// subnetIdentifierForMessage returns a human-readable string for a subnet identifier (name or UUID).
func subnetIdentifierForMessage(id capxv1.NutanixResourceIdentifier) string {
	switch {
	case id.IsUUID():
		return *id.UUID
	case id.IsName():
		return *id.Name
	default:
		return "<invalid-identifier>"
	}
}

// extractIPv4PrefixesFromSubnet extracts IPv4 prefixes from a Nutanix subnet's IP configuration.
// Returns nil for external IPAM subnets.
func extractIPv4PrefixesFromSubnet(subnet *netv4.Subnet) ([]netip.Prefix, error) {
	if len(subnet.IpConfig) == 0 {
		return nil, nil
	}

	prefixes := make([]netip.Prefix, 0, len(subnet.IpConfig))
	for i := range subnet.IpConfig {
		ipConfig := &subnet.IpConfig[i]
		if ipConfig.Ipv4 == nil {
			continue
		}
		if ipConfig.Ipv4.IpSubnet == nil {
			continue
		}
		if ipConfig.Ipv4.IpSubnet.Ip == nil || ipConfig.Ipv4.IpSubnet.Ip.Value == nil {
			continue
		}
		if ipConfig.Ipv4.IpSubnet.PrefixLength == nil {
			continue
		}

		ipValue := *ipConfig.Ipv4.IpSubnet.Ip.Value
		prefixLength := *ipConfig.Ipv4.IpSubnet.PrefixLength

		prefix, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", ipValue, prefixLength))
		if err != nil {
			return nil, fmt.Errorf("failed to parse subnet prefix %q/%d: %w", ipValue, prefixLength, err)
		}
		prefixes = append(prefixes, prefix.Masked())
	}

	return prefixes, nil
}
