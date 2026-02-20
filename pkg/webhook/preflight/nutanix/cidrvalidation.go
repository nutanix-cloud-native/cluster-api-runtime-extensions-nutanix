// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	clusterNetworkPodsFieldPath     = "$.spec.clusterNetwork.pods.cidrBlocks"
	clusterNetworkServicesFieldPath = "$.spec.clusterNetwork.services.cidrBlocks"
)

type cidrValidationCheck struct {
	cd *checkDependencies
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
