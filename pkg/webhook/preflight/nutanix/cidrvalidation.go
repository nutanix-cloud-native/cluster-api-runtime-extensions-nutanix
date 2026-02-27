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

type nodeSubnetSource struct {
	id    capxv1.NutanixResourceIdentifier
	field string
}

type resolvedNodeSubnet struct {
	prefix netip.Prefix
	field  string
	name   string
}

type cidrValidationCheck struct {
	cd *checkDependencies

	resolveNodeSubnetsFunc func(
		ctx context.Context,
		nclient client,
		sources []nodeSubnetSource,
	) ([]resolvedNodeSubnet, error)
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
	}

	serviceCIDRs, err := parseCIDRBlocks(serviceCIDRBlocks)
	if err != nil {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("Invalid Service CIDR configuration: %s", err),
			Field:   clusterNetworkServicesFieldPath,
		})
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

	nodeSubnetSources := collectNodeSubnetSources(c.cd)
	if len(nodeSubnetSources) == 0 {
		return result
	}

	if c.cd.nclient == nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: "Cannot validate subnet overlaps: Prism Central connection is not available. " +
				"Check your Nutanix credentials and ensure Prism Central is accessible.",
		})
		return result
	}

	resolveNodeSubnetsFn := c.resolveNodeSubnetsFunc
	if resolveNodeSubnetsFn == nil {
		resolveNodeSubnetsFn = resolveNodeSubnets
	}

	resolvedSubnets, err := resolveNodeSubnetsFn(ctx, c.cd.nclient, nodeSubnetSources)
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

	for _, podCIDR := range podCIDRs {
		for _, nodeSubnet := range resolvedSubnets {
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

	for _, serviceCIDR := range serviceCIDRs {
		for _, nodeSubnet := range resolvedSubnets {
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

func collectNodeSubnetSources(cd *checkDependencies) []nodeSubnetSource {
	if cd == nil {
		return nil
	}

	sources := make([]nodeSubnetSource, 0)

	if cd.nutanixClusterConfigSpec != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		for i, subnetID := range cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails.Subnets {
			sources = append(sources, nodeSubnetSource{
				id: subnetID,
				field: fmt.Sprintf(
					"$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.machineDetails.subnets[%d]", //nolint:lll // Field path is long.
					i,
				),
			})
		}
	}

	for mdName, worker := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if worker == nil || worker.Nutanix == nil {
			continue
		}
		for i, subnetID := range worker.Nutanix.MachineDetails.Subnets {
			sources = append(sources, nodeSubnetSource{
				id: subnetID,
				field: fmt.Sprintf(
					"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=workerConfig].value.nutanix.machineDetails.subnets[%d]", //nolint:lll // Field path is long.
					mdName,
					i,
				),
			})
		}
	}

	return sources
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

func resolveNodeSubnets(
	ctx context.Context,
	nclient client,
	sources []nodeSubnetSource,
) ([]resolvedNodeSubnet, error) {
	resolved := make([]resolvedNodeSubnet, 0)
	// Cache resolved prefixes by subnet key to avoid duplicate API calls
	// when the same subnet is referenced by multiple node pools.
	prefixCache := map[string][]netip.Prefix{}

	for _, src := range sources {
		cacheKey := subnetIdentifierKey(src.id)

		cached, ok := prefixCache[cacheKey]
		if !ok {
			subnets, err := getSubnets(ctx, nclient, &src.id)
			if err != nil {
				return nil, fmt.Errorf("failed to get subnet %q: %w", subnetIdentifierForMessage(src.id), err)
			}
			if len(subnets) == 0 {
				return nil, fmt.Errorf("found no subnets for identifier %q", subnetIdentifierForMessage(src.id))
			}

			prefixes := make([]netip.Prefix, 0)
			for i := range subnets {
				subnetPrefixes, err := extractIPv4PrefixesFromSubnet(&subnets[i])
				if err != nil {
					return nil, fmt.Errorf(
						"failed to extract IPv4 CIDR for subnet %q: %w",
						subnetIdentifierForMessage(src.id),
						err,
					)
				}
				prefixes = append(prefixes, subnetPrefixes...)
			}
			prefixCache[cacheKey] = prefixes
			cached = prefixes
		}

		for _, prefix := range cached {
			resolved = append(resolved, resolvedNodeSubnet{
				prefix: prefix,
				field:  src.field,
				name:   subnetIdentifierForMessage(src.id),
			})
		}
	}

	return resolved, nil
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

func extractIPv4PrefixesFromSubnet(subnet *netv4.Subnet) ([]netip.Prefix, error) {
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
