// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	clusterNetworkPodsFieldPath     = "$.spec.clusterNetwork.pods.cidrBlocks"
	clusterNetworkServicesFieldPath = "$.spec.clusterNetwork.services.cidrBlocks"
	clusterNetworkNodeSubnetsField  = "$.spec.topology.(controlPlane/workers).nutanix.machineDetails.subnets"
)

type cidrValidationCheck struct {
	cd *checkDependencies

	resolveSubnetPrefixesFunc func(
		ctx context.Context,
		nclient client,
		ids []capxv1.NutanixResourceIdentifier,
	) ([]netip.Prefix, error)
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
	if c.cd.cluster.Spec.ClusterNetwork != nil {
		if c.cd.cluster.Spec.ClusterNetwork.Pods != nil {
			podCIDRBlocks = c.cd.cluster.Spec.ClusterNetwork.Pods.CIDRBlocks
		}
		if c.cd.cluster.Spec.ClusterNetwork.Services != nil {
			serviceCIDRBlocks = c.cd.cluster.Spec.ClusterNetwork.Services.CIDRBlocks
		}
	}

	podCIDRs, err := parseCIDRBlocks(podCIDRBlocks)
	if err != nil {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("Invalid Pod CIDR configuration: %s", err),
			Field:   clusterNetworkPodsFieldPath,
		})
		return result
	}

	serviceCIDRs, err := parseCIDRBlocks(serviceCIDRBlocks)
	if err != nil {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("Invalid Service CIDR configuration: %s", err),
			Field:   clusterNetworkServicesFieldPath,
		})
		return result
	}

	applyPodCIDRValidation(&result, podCIDRs)
	applyServiceCIDRValidation(&result, serviceCIDRs)

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

	subnetIDs := collectSubnetIdentifiers(c.cd)
	if len(subnetIDs) == 0 {
		return result
	}

	if c.cd.nclient == nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: "Cannot validate subnet overlaps: Prism Central connection is not available. " +
				"Check your Nutanix credentials and ensure Prism Central is accessible.",
			Field: clusterNetworkNodeSubnetsField,
		})
		return result
	}

	resolveSubnetPrefixesFn := c.resolveSubnetPrefixesFunc
	if resolveSubnetPrefixesFn == nil {
		resolveSubnetPrefixesFn = resolveSubnetPrefixes
	}

	nodeNetworkCIDRs, err := resolveSubnetPrefixesFn(ctx, c.cd.nclient, subnetIDs)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to resolve node subnet CIDRs from Prism Central: %s. This is usually a temporary error. Please retry.",
				err,
			),
			Field: clusterNetworkNodeSubnetsField,
		})
		return result
	}

	for _, podCIDR := range podCIDRs {
		for _, nodeCIDR := range nodeNetworkCIDRs {
			if prefixesOverlap(podCIDR, nodeCIDR) {
				result.Allowed = false
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf(
						"Pod CIDR %q overlaps with node subnet CIDR %q. Use non-overlapping ranges and retry.",
						podCIDR.String(),
						nodeCIDR.String(),
					),
					Field: clusterNetworkPodsFieldPath,
				})
			}
		}
	}

	for _, serviceCIDR := range serviceCIDRs {
		for _, nodeCIDR := range nodeNetworkCIDRs {
			if prefixesOverlap(serviceCIDR, nodeCIDR) {
				result.Allowed = false
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf(
						"Service CIDR %q overlaps with node subnet CIDR %q. Use non-overlapping ranges and retry.",
						serviceCIDR.String(),
						nodeCIDR.String(),
					),
					Field: clusterNetworkServicesFieldPath,
				})
			}
		}
	}

	return result
}

func newCIDRValidationChecks(cd *checkDependencies) []preflight.Check {
	if cd == nil || cd.cluster == nil {
		return nil
	}

	return []preflight.Check{
		&cidrValidationCheck{
			cd: cd,
		},
	}
}

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

func applyPodCIDRValidation(result *preflight.CheckResult, prefixes []netip.Prefix) {
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

func applyServiceCIDRValidation(result *preflight.CheckResult, prefixes []netip.Prefix) {
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

func maxNodesForPodCIDR(podMaskSize, nodeMaskSize int) int {
	if podMaskSize >= nodeMaskSize {
		return 1
	}
	return 1 << (nodeMaskSize - podMaskSize)
}

func prefixesOverlap(a, b netip.Prefix) bool {
	return a.Overlaps(b)
}

func collectSubnetIdentifiers(cd *checkDependencies) []capxv1.NutanixResourceIdentifier {
	if cd == nil {
		return nil
	}

	ids := make([]capxv1.NutanixResourceIdentifier, 0)
	seen := map[string]struct{}{}

	appendUnique := func(id capxv1.NutanixResourceIdentifier) {
		key := subnetIdentifierKey(id)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		ids = append(ids, id)
	}

	if cd.nutanixClusterConfigSpec != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		for _, subnetID := range cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails.Subnets {
			appendUnique(subnetID)
		}
	}

	for _, worker := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if worker == nil || worker.Nutanix == nil {
			continue
		}
		for _, subnetID := range worker.Nutanix.MachineDetails.Subnets {
			appendUnique(subnetID)
		}
	}

	return ids
}

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

func resolveSubnetPrefixes(
	ctx context.Context,
	nclient client,
	ids []capxv1.NutanixResourceIdentifier,
) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0)

	for _, id := range ids {
		subnets, err := getSubnets(ctx, nclient, &id)
		if err != nil {
			return nil, fmt.Errorf("failed to get subnet %q: %w", subnetIdentifierForMessage(id), err)
		}
		if len(subnets) == 0 {
			return nil, fmt.Errorf("found no subnets for identifier %q", subnetIdentifierForMessage(id))
		}

		for i := range subnets {
			subnetPrefixes, err := extractIPv4PrefixesFromSubnet(subnets[i])
			if err != nil {
				return nil, fmt.Errorf(
					"failed to extract IPv4 CIDR for subnet %q: %w",
					subnetIdentifierForMessage(id),
					err,
				)
			}
			prefixes = append(prefixes, subnetPrefixes...)
		}
	}

	slices.SortFunc(prefixes, func(a, b netip.Prefix) int {
		return a.Addr().Compare(b.Addr())
	})
	prefixes = slices.Compact(prefixes)

	return prefixes, nil
}

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

func extractIPv4PrefixesFromSubnet(subnet netv4.Subnet) ([]netip.Prefix, error) {
	if len(subnet.IpConfig) == 0 {
		return nil, fmt.Errorf("subnet has no ipConfig")
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

	if len(prefixes) == 0 {
		return nil, fmt.Errorf("subnet has no IPv4 CIDR in ipConfig")
	}

	return prefixes, nil
}
