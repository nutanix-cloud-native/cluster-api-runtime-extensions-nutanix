// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"
	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"k8s.io/utils/ptr"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

// autoIPAMField is the topology path operators see in a failure message.
const autoIPAMField = `$.spec.topology.variables[?@.name=="clusterConfig"].value.addons.serviceLoadBalancer.configuration.autoIPAM` //nolint:lll // The field path is long.

// reservedIPsProbeLimit keeps the probe to one row. It is a permission and
// capability probe, not a listing.
const reservedIPsProbeLimit = 1

type serviceLoadBalancerAutoIPAMCheck struct {
	autoIPAM *carenv1.ServiceLoadBalancerAutoIPAM
	// prismElementCluster resolves the subnet when it is given by name.
	prismElementCluster string
	nclient             client
}

func (c *serviceLoadBalancerAutoIPAMCheck) Name() string {
	return "NutanixServiceLoadBalancerAutoIPAM"
}

// Run proves that the CCM will be able to reserve a Service address on the
// nominated subnet, at create time rather than when the first LoadBalancer
// Service appears.
//
// A single reserved-IP listing answers every question that matters:
//
//   - the subnet exists and these credentials can see it, which a
//     project-scoped Prism Central can make false for a subnet that exists;
//   - the subnet is IPAM-managed. An unmanaged subnet has no address pool and
//     Prism answers NETWORKING-10056 "Cannot fetch reserved IP addresses for
//     unmanaged subnets" (measured on PC 7.6 GA). Reserving from it can never
//     work, and without this check the cluster comes up and then every
//     LoadBalancer Service hangs in Pending;
//   - the credentials carry the privilege to read reservations, which is
//     refused synchronously. The reserve itself answers 202 and fails later in
//     a task, so a permission gap is otherwise invisible until provisioning.
func (c *serviceLoadBalancerAutoIPAMCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}

	if c.autoIPAM.Subnet == "" {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: "Service LoadBalancer automatic IP address management requires a subnet. Review the Cluster.", //nolint:lll // Message is long.
			Field:   autoIPAMField + ".subnet",
		})
		return result
	}

	// A name has to be resolved before anything can be asked about the subnet.
	// A UUID is used as given: the listing below reports it if it does not exist.
	subnetExtID := c.autoIPAM.Subnet
	if _, err := uuid.Parse(subnetExtID); err != nil {
		resolved, resolveErr := c.resolveSubnetByName(ctx, c.autoIPAM.Subnet)
		if resolveErr != nil {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to find the Service LoadBalancer subnet %q: %s. Review the Cluster.",
					c.autoIPAM.Subnet, resolveErr,
				),
				Field: autoIPAMField + ".subnet",
			})
			return result
		}
		subnetExtID = resolved
	}

	if err := c.nclient.ListReservedIpsBySubnetId(ctx, subnetExtID, ptr.To(reservedIPsProbeLimit)); err != nil {
		result.Allowed = false
		switch {
		case isUnmanagedSubnetError(err):
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Subnet %q is not IP address managed, so the Nutanix CCM cannot reserve addresses from it. Use an IPAM-managed subnet, or configure static addressRanges instead. (%s)", //nolint:lll // Message is long.
					c.autoIPAM.Subnet,
					err,
				),
				Field: autoIPAMField + ".subnet",
			})
		case isSubnetNotFoundError(err):
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Subnet %q does not exist on this Prism Central, or these credentials cannot see it. Review the Cluster. (%s)", //nolint:lll // Message is long.
					c.autoIPAM.Subnet,
					err,
				),
				Field: autoIPAMField + ".subnet",
			})
		case isPermissionError(err):
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"The Prism Central credentials are not permitted to read IP reservations on subnet %q, so the Nutanix CCM cannot reserve addresses from it. Grant the role the IP reservation privileges on this subnet. (%s)", //nolint:lll // Message is long.
					c.autoIPAM.Subnet,
					err,
				),
				Field: autoIPAMField + ".subnet",
			})
		default:
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to read IP reservations on subnet %q: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
					c.autoIPAM.Subnet,
					err,
				),
				Field: autoIPAMField + ".subnet",
			})
		}
		return result
	}

	return result
}

// resolveSubnetByName resolves a subnet name to its extID, requiring exactly
// one match. The Prism Element cluster narrows the search when it is set.
func (c *serviceLoadBalancerAutoIPAMCheck) resolveSubnetByName(
	ctx context.Context,
	name string,
) (string, error) {
	// A subnet reports its cluster as a UUID, so a Prism Element given by name
	// has to be resolved first or the narrowing below would never match. This is
	// the common case: the CLI passes the name the operator typed.
	clusterExtID := ""
	if c.prismElementCluster != "" {
		resolved, err := c.resolveClusterExtID(ctx, c.prismElementCluster)
		if err != nil {
			return "", err
		}
		clusterExtID = resolved
	}

	subnets, err := c.listSubnetsByName(ctx, name)
	if err != nil {
		return "", err
	}

	matched := make([]string, 0, 1)
	for i := range subnets {
		subnet := &subnets[i]
		// The name is compared here as well as in the filter, so the result never
		// depends on the server's matching rules.
		if subnet.Name == nil || *subnet.Name != name || subnet.ExtId == nil || *subnet.ExtId == "" {
			continue
		}
		if clusterExtID != "" && subnetDeclaresCluster(subnet) &&
			!subnetMatchesCluster(subnet, clusterExtID) {
			continue
		}
		matched = append(matched, *subnet.ExtId)
	}

	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return "", fmt.Errorf("no subnet named %q", name)
	default:
		return "", fmt.Errorf("the name matches %d subnets (%s); use the subnet UUID instead",
			len(matched), strings.Join(matched, ", "))
	}
}

// listSubnetsByName lists candidate subnets for a name.
//
// The server-side filter is only an optimisation. Prism Central 7.6 answers
// HTTP 400 NETWORKING-10002 "Error while parsing URI" for a filter holding any
// non-ASCII character, so a name it cannot express falls back to a full list.
// Without the fallback this check would reject a subnet the CCM resolves
// perfectly well, blocking a cluster that would have worked.
func (c *serviceLoadBalancerAutoIPAMCheck) listSubnetsByName(
	ctx context.Context,
	name string,
) ([]netv4.Subnet, error) {
	if isASCIIName(name) {
		filter := fmt.Sprintf("name eq '%s'", strings.ReplaceAll(name, "'", "''"))
		if subnets, err := c.subnetList(ctx, ptr.To(filter)); err == nil {
			return subnets, nil
		}
	}
	return c.subnetList(ctx, nil)
}

func (c *serviceLoadBalancerAutoIPAMCheck) subnetList(
	ctx context.Context,
	filter *string,
) ([]netv4.Subnet, error) {
	resp, err := c.nclient.ListSubnets(ctx, nil, nil, filter, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list subnets: %w", err)
	}
	if resp == nil || resp.GetData() == nil {
		return nil, nil
	}
	subnets, ok := resp.GetData().([]netv4.Subnet)
	if !ok {
		return nil, fmt.Errorf("unexpected subnet list data type %T", resp.GetData())
	}
	return subnets, nil
}

// resolveClusterExtID resolves a Prism Element name to its extID. A value that
// is already a UUID is used as given.
func (c *serviceLoadBalancerAutoIPAMCheck) resolveClusterExtID(
	ctx context.Context,
	clusterExtIDOrName string,
) (string, error) {
	if _, err := uuid.Parse(clusterExtIDOrName); err == nil {
		return clusterExtIDOrName, nil
	}
	resp, err := c.nclient.ListClusters(ctx, nil, nil, nil, nil, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list clusters: %w", err)
	}
	if resp == nil || resp.GetData() == nil {
		return "", fmt.Errorf("no cluster named %q", clusterExtIDOrName)
	}
	clusters, ok := resp.GetData().([]clustermgmtv4.Cluster)
	if !ok {
		return "", fmt.Errorf("unexpected cluster list data type %T", resp.GetData())
	}
	matched := make([]string, 0, 1)
	for i := range clusters {
		if clusters[i].Name != nil && *clusters[i].Name == clusterExtIDOrName &&
			clusters[i].ExtId != nil && *clusters[i].ExtId != "" {
			matched = append(matched, *clusters[i].ExtId)
		}
	}
	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return "", fmt.Errorf("no Prism Element cluster named %q", clusterExtIDOrName)
	default:
		return "", fmt.Errorf("the Prism Element cluster name %q matches %d clusters; use its UUID instead",
			clusterExtIDOrName, len(matched))
	}
}

func isASCIIName(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func subnetDeclaresCluster(subnet *netv4.Subnet) bool {
	return (subnet.ClusterReference != nil && *subnet.ClusterReference != "") ||
		len(subnet.ClusterReferenceList) > 0
}

// subnetMatchesCluster compares against the cluster extID. The caller resolves
// a cluster name first, because a subnet reports its cluster as a UUID.
func subnetMatchesCluster(subnet *netv4.Subnet, clusterExtID string) bool {
	if subnet.ClusterReference != nil && *subnet.ClusterReference == clusterExtID {
		return true
	}
	return slices.Contains(subnet.ClusterReferenceList, clusterExtID)
}

// isUnmanagedSubnetError reports whether Prism refused because the subnet has
// no IP address pool. Measured on PC 7.6 GA.
func isUnmanagedSubnetError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unmanaged subnet") ||
		strings.Contains(message, "networking-10056")
}

// isSubnetNotFoundError reports whether the subnet does not exist, or is not
// visible to these credentials, which a project-scoped Prism Central can make
// true for a subnet that does exist. Prism answers NETWORKING-10060
// "Existing subnet: <uuid> is not found" (measured on PC 7.6 GA). Either way
// it is a configuration error and not worth retrying.
func isSubnetNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "networking-10060") ||
		strings.Contains(message, "knotfound") ||
		strings.Contains(message, "is not found") ||
		strings.Contains(message, "not found")
}

// isPermissionError reports whether Prism refused the request rather than
// failing it, which is how a missing IP reservation privilege presents.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "forbidden") ||
		strings.Contains(message, "unauthorized") ||
		strings.Contains(message, "not authorized") ||
		strings.Contains(message, "permission") ||
		strings.Contains(message, "403") ||
		strings.Contains(message, "401")
}

// newServiceLoadBalancerAutoIPAMChecks returns the automatic-addressing checks
// for the cluster. A cluster with static address ranges, or no Service
// LoadBalancer at all, gets none.
func newServiceLoadBalancerAutoIPAMChecks(cd *checkDependencies) []preflight.Check {
	if cd.nutanixClusterConfigSpec == nil || cd.nclient == nil {
		return nil
	}
	addons := cd.nutanixClusterConfigSpec.Addons
	if addons == nil || addons.ServiceLoadBalancer == nil ||
		addons.ServiceLoadBalancer.Configuration == nil ||
		addons.ServiceLoadBalancer.Configuration.AutoIPAM == nil {
		return nil
	}

	autoIPAM := addons.ServiceLoadBalancer.Configuration.AutoIPAM
	prismElementCluster := ""
	if autoIPAM.Cluster != nil {
		prismElementCluster = *autoIPAM.Cluster
	}

	return []preflight.Check{
		&serviceLoadBalancerAutoIPAMCheck{
			autoIPAM:            autoIPAM,
			prismElementCluster: prismElementCluster,
			nclient:             cd.nclient,
		},
	}
}
