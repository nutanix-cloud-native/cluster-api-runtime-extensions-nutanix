// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	metroPrismElementCount = 2
	nutanixMetroName       = "NutanixMetro"
	cpFailureDomainsField  = "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.failureDomains" //nolint:lll // Message is long.
	// metroMaxRTTMillis is the maximum network round-trip time, in
	// milliseconds, supported between the two Prism Elements of a metro
	// configuration for safe synchronous replication.
	metroMaxRTTMillis = 5.0
)

type metroCheck struct {
	metroName  string
	namespace  string
	field      string
	kclient    ctrlclient.Client
	nclient    client
	errMessage *string
}

func failCheck(result *preflight.CheckResult, field, message string) {
	result.Allowed = false
	result.Causes = append(result.Causes, preflight.Cause{
		Message: message,
		Field:   field,
	})
}

func failCheckInternal(result *preflight.CheckResult, field, message string) {
	result.InternalError = true
	failCheck(result, field, message)
}

func (mc *metroCheck) Name() string {
	return nutanixMetroName
}

func newMetroChecks(cd *checkDependencies) []preflight.Check {
	checks := []preflight.Check{}

	if cd == nil || cd.kclient == nil || cd.nclient == nil || cd.pcVersion == "" {
		return checks
	}

	// Build the set of NutanixMetro objects referenced by the Cluster
	refs := referencedMetros(cd)

	metroNames := []string{}
	var firstField string
	for _, ref := range refs {
		checks = append(checks, &metroCheck{
			metroName:  ref.metroName,
			namespace:  cd.cluster.Namespace,
			field:      ref.field,
			kclient:    cd.kclient,
			nclient:    cd.nclient,
			errMessage: ref.errMessage,
		})

		// Only count successfully resolved metros towards the single-metro rule.
		if ref.errMessage == nil {
			if firstField == "" {
				firstField = ref.field
			}
			metroNames = append(metroNames, ref.metroName)
		}
	}

	// A Cluster must be backed by exactly one NutanixMetro: a metro is a
	// two-Prism-Element relationship, so referencing more than one would make the
	// Cluster span more than two Prism Elements.
	if len(metroNames) > 1 {
		checks = append(checks, &singleMetroCheck{
			metroNames: metroNames,
			field:      firstField,
		})
	}

	// For a metro Cluster, all Control Plane and Worker nodes must span exactly
	// two Prism Elements in total, across every failure domain the Cluster uses
	// (including standalone NutanixFailureDomains that are not backed by a
	// NutanixMetro). This catches, for example, a MachineDeployment placed on a
	// standalone failure domain whose Prism Element is a third PE.
	if len(metroNames) > 0 {
		fdNames, errMessage := clusterFailureDomainNames(cd)
		checks = append(checks,
			&clusterPrismElementScaleCheck{
				failureDomainNames: fdNames,
				namespace:          cd.cluster.Namespace,
				field:              firstField,
				kclient:            cd.kclient,
				nclient:            cd.nclient,
				errMessage:         errMessage,
			},
			// Prism Central must not reside on either of the metro's Prism
			// Elements: otherwise a single Prism Element failure would take down
			// Prism Central together with half of the metro.
			&prismCentralMetroHostingCheck{
				failureDomainNames: fdNames,
				namespace:          cd.cluster.Namespace,
				field:              firstField,
				kclient:            cd.kclient,
				nclient:            cd.nclient,
				errMessage:         errMessage,
			},
			// The network round-trip time between the two metro Prism Elements
			// must be within the supported bound for safe synchronous
			// replication.
			&metroReplicationLatencyCheck{
				failureDomainNames: fdNames,
				namespace:          cd.cluster.Namespace,
				field:              firstField,
				kclient:            cd.kclient,
				nclient:            cd.nclient,
				errMessage:         errMessage,
			},
			// Every Control Plane and Worker node pool must be placed on a
			// metro failure domain: otherwise only some node pools would be
			// protected by synchronous replication and the Cluster would be
			// only partially HA at the infrastructure level.
			&allNodePoolsMetroCheck{
				nodePools: clusterNodePools(cd),
				field:     firstField,
			},
		)
	}

	return checks
}

// nodePool describes a single Control Plane or Worker node pool and the failure
// domains it is configured with. failureDomains is empty when the node pool has
// no failure domain configured.
type nodePool struct {
	description    string
	failureDomains []string
	field          string
}

// clusterNodePools enumerates every Control Plane and Worker node pool of the
// Cluster, preserving node pools that have no failure domain configured (which
// forEachClusterFailureDomain intentionally skips).
func clusterNodePools(cd *checkDependencies) []nodePool {
	pools := []nodePool{}
	if cd == nil {
		return pools
	}

	if cd.nutanixClusterConfigSpec != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		fds := []string{}
		for _, fd := range cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.FailureDomains {
			if fd != "" {
				fds = append(fds, fd)
			}
		}
		pools = append(pools, nodePool{
			description:    "The Control Plane",
			failureDomains: fds,
			field:          cpFailureDomainsField,
		})
	}

	if cd.cluster != nil && cd.cluster.Spec.Topology.IsDefined() {
		for i := range cd.cluster.Spec.Topology.Workers.MachineDeployments {
			md := &cd.cluster.Spec.Topology.Workers.MachineDeployments[i]
			fds := []string{}
			if md.FailureDomain != "" {
				fds = append(fds, md.FailureDomain)
			}
			pools = append(pools, nodePool{
				description:    fmt.Sprintf("Worker MachineDeployment %q", md.Name),
				failureDomains: fds,
				field: fmt.Sprintf(
					"$.spec.topology.workers.machineDeployments[?@.name==%q].failureDomain",
					md.Name,
				),
			})
		}
	}

	return pools
}

// allNodePoolsMetroCheck enforces that, when the Cluster uses metro, every
// Control Plane and Worker node pool is configured with a NutanixMetro or
// NutanixMetroSite failure domain. This prevents a partially-protected Cluster
// where only some node pools benefit from synchronous replication.
type allNodePoolsMetroCheck struct {
	nodePools []nodePool
	field     string
}

func (c *allNodePoolsMetroCheck) Name() string {
	return nutanixMetroName
}

func (c *allNodePoolsMetroCheck) Run(_ context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}

	for _, np := range c.nodePools {
		if len(np.failureDomains) == 0 {
			failCheck(&result, np.field, fmt.Sprintf(
				"%s is not configured with a failure domain. In a metro configuration, every Control Plane and Worker node pool must be placed on a NutanixMetro or NutanixMetroSite failure domain so that all nodes are protected by synchronous replication. Configure it with a NutanixMetro or NutanixMetroSite failure domain and retry.", //nolint:lll // Message is long.
				np.description,
			))
			continue
		}

		for _, fd := range np.failureDomains {
			if isNutanixMetroFailureDomain(fd) || isNutanixMetroSiteFailureDomain(fd) {
				continue
			}
			failCheck(&result, np.field, fmt.Sprintf(
				"%s uses failure domain %q, which is not a NutanixMetro or NutanixMetroSite failure domain. In a metro configuration, every Control Plane and Worker node pool must be placed on a NutanixMetro or NutanixMetroSite failure domain so that all nodes are protected by synchronous replication. Use a NutanixMetro or NutanixMetroSite failure domain and retry.", //nolint:lll // Message is long.
				np.description,
				fd,
			))
		}
	}

	return result
}

// clusterFailureDomainNames returns the distinct NutanixFailureDomain names used
// by the Cluster across its control plane and worker failure domains, resolving
// NutanixMetro and NutanixMetroSite references to their underlying failure
// domains. The returned string is a non-nil error message if any reference could
// not be resolved.
func clusterFailureDomainNames(cd *checkDependencies) (names []string, errMessage *string) {
	nameSet := map[string]struct{}{}

	add := func(fd string) {
		names, err := getFailureDomainNames(cd, fd)
		if err != nil {
			if errMessage == nil {
				errMessage = ptr.To(err.Error())
			}
			return
		}
		for _, n := range names {
			if n != "" {
				nameSet[n] = struct{}{}
			}
		}
	}

	forEachClusterFailureDomain(cd, func(fd, _ string) {
		add(fd)
	})

	names = make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, errMessage
}

// forEachClusterFailureDomain iterates over every non-empty failure domain
// referenced by the Cluster's control plane and worker machine deployments.
func forEachClusterFailureDomain(cd *checkDependencies, visit func(fd, field string)) {
	if cd == nil || visit == nil {
		return
	}

	if cd.nutanixClusterConfigSpec != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		for _, fd := range cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.FailureDomains {
			if fd != "" {
				visit(fd, cpFailureDomainsField)
			}
		}
	}

	if cd.cluster != nil && cd.cluster.Spec.Topology.IsDefined() {
		for i := range cd.cluster.Spec.Topology.Workers.MachineDeployments {
			md := &cd.cluster.Spec.Topology.Workers.MachineDeployments[i]
			if md.FailureDomain == "" {
				continue
			}
			visit(md.FailureDomain, fmt.Sprintf(
				"$.spec.topology.workers.machineDeployments[?@.name==%q].failureDomain",
				md.Name,
			))
		}
	}
}

// clusterPrismElementScaleCheck enforces that a metro Cluster's Control Plane and
// Worker nodes span exactly two distinct Prism Elements in total, across every
// failure domain the Cluster uses.
type clusterPrismElementScaleCheck struct {
	failureDomainNames []string
	namespace          string
	field              string
	kclient            ctrlclient.Client
	nclient            client
	errMessage         *string
}

func (c *clusterPrismElementScaleCheck) Name() string {
	return nutanixMetroName
}

func (c *clusterPrismElementScaleCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}

	if c.errMessage != nil {
		failCheck(&result, c.field, fmt.Sprintf(
			"Failed to determine the Prism Elements the Cluster spans: %s",
			*c.errMessage,
		))
		return result
	}

	peUUIDs := map[string]struct{}{}
	for _, fdName := range c.failureDomainNames {
		peUUID, err := failureDomainPrismElementUUID(ctx, c.kclient, c.nclient, c.namespace, fdName)
		if err != nil {
			// The specific failure domain is validated in detail by other checks;
			// here we only report that the span could not be computed.
			failCheckInternal(&result, c.field, fmt.Sprintf(
				"Failed to determine the Prism Element of Failure Domain %q used by the Cluster: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
				fdName,
				err,
			))
			return result
		}
		peUUIDs[peUUID] = struct{}{}
	}

	if len(peUUIDs) != metroPrismElementCount {
		failCheck(&result, c.field, fmt.Sprintf(
			"The Cluster's Control Plane and Worker nodes must span exactly %d distinct Prism Elements in a metro configuration, but they span %d across failure domains %s. Ensure all nodes are placed on the two Prism Elements of a single NutanixMetro.", //nolint:lll // Message is long.
			metroPrismElementCount,
			len(peUUIDs),
			strings.Join(c.failureDomainNames, ", "),
		))
	}

	return result
}

// failureDomainPrismElementUUID resolves the Prism Element UUID of a single
// NutanixFailureDomain by name.
func failureDomainPrismElementUUID(
	ctx context.Context,
	kclient ctrlclient.Client,
	nclient client,
	namespace string,
	fdName string,
) (string, error) {
	fdObj := &capxv1.NutanixFailureDomain{}
	fdKey := ctrlclient.ObjectKey{Name: fdName, Namespace: namespace}
	if err := kclient.Get(ctx, fdKey, fdObj); err != nil {
		return "", fmt.Errorf("failed to get NutanixFailureDomain %q: %w", fdName, err)
	}

	peIdentifier := fdObj.Spec.PrismElementCluster
	peClusters, err := getClusters(ctx, nclient, &peIdentifier)
	if err != nil {
		return "", fmt.Errorf("failed to get Prism Element cluster %q: %w", peIdentifier, err)
	}
	if len(peClusters) != 1 {
		return "", fmt.Errorf(
			"found %d Prism Element cluster(s) matching identifier %q, expected exactly 1",
			len(peClusters),
			peIdentifier,
		)
	}
	if peClusters[0].ExtId == nil {
		return "", fmt.Errorf("no ExtId returned for Prism Element cluster %q", peIdentifier)
	}
	return *peClusters[0].ExtId, nil
}

// prismCentralMetroHostingCheck enforces that Prism Central does not reside on
// any Prism Element used by the metro Cluster. If Prism Central were hosted on
// one of the two metro Prism Elements, a failure of that Prism Element would
// take Prism Central down together with half of the metro.
type prismCentralMetroHostingCheck struct {
	failureDomainNames []string
	namespace          string
	field              string
	kclient            ctrlclient.Client
	nclient            client
	errMessage         *string
}

func (c *prismCentralMetroHostingCheck) Name() string {
	return nutanixMetroName
}

func (c *prismCentralMetroHostingCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}

	if c.errMessage != nil {
		failCheck(&result, c.field, fmt.Sprintf(
			"Failed to determine the Prism Elements used for metro: %s",
			*c.errMessage,
		))
		return result
	}

	hostingPEUUID, err := c.nclient.GetPrismCentralHostingClusterExtID(ctx)
	if err != nil {
		failCheckInternal(&result, c.field, fmt.Sprintf(
			"Failed to determine the Prism Element hosting Prism Central: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
			err,
		))
		return result
	}
	// If the hosting Prism Element cannot be determined, do not block the
	// Cluster: we have no evidence of a violation.
	if hostingPEUUID == "" {
		return result
	}

	for _, fdName := range c.failureDomainNames {
		peUUID, err := failureDomainPrismElementUUID(ctx, c.kclient, c.nclient, c.namespace, fdName)
		if err != nil {
			failCheckInternal(&result, c.field, fmt.Sprintf(
				"Failed to determine the Prism Element of Failure Domain %q used for metro: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
				fdName,
				err,
			))
			return result
		}
		if peUUID == hostingPEUUID {
			failCheck(&result, c.field, fmt.Sprintf(
				"Prism Central resides on Prism Element %q, which is used for metro by Failure Domain %q. Prism Central must not reside on either of the two Prism Elements used for metro, so that a Prism Element failure does not take down Prism Central together with half of the metro. Move Prism Central to a different Prism Element and retry.", //nolint:lll // Message is long.
				hostingPEUUID,
				fdName,
			))
			return result
		}
	}

	return result
}

// metroReplicationLatencyCheck enforces that the network round-trip time
// between the two Prism Elements used for metro is within the supported bound
// for safe synchronous replication.
type metroReplicationLatencyCheck struct {
	failureDomainNames []string
	namespace          string
	field              string
	kclient            ctrlclient.Client
	nclient            client
	errMessage         *string
}

func (c *metroReplicationLatencyCheck) Name() string {
	return nutanixMetroName
}

func (c *metroReplicationLatencyCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}

	if c.errMessage != nil {
		failCheck(&result, c.field, fmt.Sprintf(
			"Failed to determine the Prism Elements used for metro: %s",
			*c.errMessage,
		))
		return result
	}

	peUUIDs := c.distinctPrismElementUUIDs(ctx, &result)
	if !result.Allowed {
		return result
	}

	// The round-trip time is only defined between exactly two Prism Elements.
	// A different count is a separate violation reported by the PE-scale check,
	// so there is nothing to validate here.
	if len(peUUIDs) != metroPrismElementCount {
		return result
	}

	rttMillis, found, err := c.nclient.GetInterClusterRTTMillis(ctx, peUUIDs[0], peUUIDs[1])
	if err != nil {
		failCheckInternal(&result, c.field, fmt.Sprintf(
			"Failed to determine the network round-trip time between the Prism Elements used for metro: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
			err,
		))
		return result
	}
	// If the round-trip time cannot be determined, do not block the Cluster: we
	// have no evidence of a violation.
	if !found {
		return result
	}

	if rttMillis > metroMaxRTTMillis {
		failCheck(&result, c.field, fmt.Sprintf(
			"The network round-trip time between the two Prism Elements used for metro is %.2fms, which exceeds the supported maximum of %.0fms. Synchronous metro replication requires a round-trip time within %.0fms. Use Prism Elements with lower network latency between them and retry.", //nolint:lll // Message is long.
			rttMillis,
			metroMaxRTTMillis,
			metroMaxRTTMillis,
		))
	}

	return result
}

// distinctPrismElementUUIDs resolves the distinct Prism Element UUIDs of the
// Cluster's failure domains, preserving order. It appends a cause and marks the
// result on resolution failure.
func (c *metroReplicationLatencyCheck) distinctPrismElementUUIDs(
	ctx context.Context,
	result *preflight.CheckResult,
) []string {
	seen := map[string]struct{}{}
	peUUIDs := []string{}
	for _, fdName := range c.failureDomainNames {
		peUUID, err := failureDomainPrismElementUUID(ctx, c.kclient, c.nclient, c.namespace, fdName)
		if err != nil {
			failCheckInternal(result, c.field, fmt.Sprintf(
				"Failed to determine the Prism Element of Failure Domain %q used for metro: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
				fdName,
				err,
			))
			return nil
		}
		if _, ok := seen[peUUID]; ok {
			continue
		}
		seen[peUUID] = struct{}{}
		peUUIDs = append(peUUIDs, peUUID)
	}
	return peUUIDs
}

// singleMetroCheck enforces that a Cluster references at most one NutanixMetro.
type singleMetroCheck struct {
	metroNames []string
	field      string
}

func (c *singleMetroCheck) Name() string {
	return nutanixMetroName
}

func (c *singleMetroCheck) Run(_ context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}

	if len(c.metroNames) > 1 {
		names := append([]string(nil), c.metroNames...)
		sort.Strings(names)
		failCheck(&result, c.field, fmt.Sprintf(
			"A Cluster may reference at most one NutanixMetro, but it references %d: %s. Control Plane and Worker nodes in a metro configuration must span exactly two Prism Elements from a single NutanixMetro.", //nolint:lll // Message is long.
			len(names),
			strings.Join(names, ", "),
		))
	}

	return result
}

// metroReference identifies a NutanixMetro referenced by the Cluster and the
// topology field that referenced it.
type metroReference struct {
	metroName  string
	field      string
	errMessage *string
}

// referencedMetros returns the distinct NutanixMetro objects referenced by the
// Cluster's control plane and worker failure domains.
func referencedMetros(cd *checkDependencies) []metroReference {
	seen := map[string]struct{}{}
	refs := []metroReference{}

	addMetro := func(fd, field string) {
		metroName, err := metroNameFromFailureDomain(cd, fd)
		if err != nil {
			// Surface the resolution error through a check so the user sees it.
			key := "err:" + fd
			if _, ok := seen[key]; ok {
				return
			}
			seen[key] = struct{}{}
			cd.log.Error(err, fmt.Sprintf("failed to resolve NutanixMetro for failureDomain %s", fd))
			refs = append(refs, metroReference{
				metroName:  fd,
				field:      field,
				errMessage: ptr.To(err.Error()),
			})
			return
		}
		if metroName == "" {
			return
		}
		if _, ok := seen[metroName]; ok {
			return
		}
		seen[metroName] = struct{}{}
		refs = append(refs, metroReference{metroName: metroName, field: field})
	}

	forEachClusterFailureDomain(cd, addMetro)

	return refs
}

// metroNameFromFailureDomain resolves the NutanixMetro name referenced by a
// failure domain string. It returns an empty name (and nil error) for failure
// domains that are not backed by a NutanixMetro.
func metroNameFromFailureDomain(cd *checkDependencies, fd string) (string, error) {
	ctx := context.TODO()
	namespace := cd.cluster.Namespace

	switch {
	case isNutanixMetroFailureDomain(fd):
		return fd[len(metroFailureDomainPrefix):], nil
	case isNutanixMetroSiteFailureDomain(fd):
		metroSiteName := fd[len(metroSiteFailureDomainPrefix):]
		metroSiteObj := &capxv1.NutanixMetroSite{}
		key := ctrlclient.ObjectKey{Name: metroSiteName, Namespace: namespace}
		if err := cd.kclient.Get(ctx, key, metroSiteObj); err != nil {
			return "", fmt.Errorf(
				"failed to fetch the NutanixMetroSite %s referenced by failureDomain %s: %w",
				metroSiteName,
				fd,
				err,
			)
		}
		return metroSiteObj.Spec.MetroRef.Name, nil
	default:
		return "", nil
	}
}

func (mc *metroCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}

	if mc.errMessage != nil {
		failCheck(&result, mc.field, fmt.Sprintf("Failed to check NutanixMetro %q: %s", mc.metroName, *mc.errMessage))
		return result
	}

	metroObj, ok := mc.getMetro(ctx, &result)
	if !ok {
		return result
	}

	// Resolve each failure domain to its Prism Element and the network layer of
	// its subnets. peUUIDs de-duplicates Prism Elements so that two failure
	// domains pointing at the same PE are detected. subnetAttrs collects the
	// distinct network attributes (layer, VLAN ID/VNI and CIDR) observed across
	// all failure-domain subnets.
	subnetAttrs := newMetroSubnetAttributes()
	peUUIDs := mc.evaluateFailureDomains(ctx, metroObj, subnetAttrs, &result)
	if !result.Allowed {
		return result
	}

	// Guardrail: PE scale limit. The metro's failure domains must span exactly
	// two distinct Prism Elements.
	if len(peUUIDs) != metroPrismElementCount {
		failCheck(&result, mc.field, fmt.Sprintf(
			"NutanixMetro %q must span exactly %d distinct Prism Elements, but its failure domains resolve to %d. Control Plane and Worker nodes in a metro configuration must span exactly two Prism Elements.", //nolint:lll // Message is long.
			mc.metroName,
			metroPrismElementCount,
			len(peUUIDs),
		))
	}

	// Guardrail: network/subnet consistency. Subnets across the failure domains
	// must reside on the same network layer (all VLAN-tagged, or all Overlay),
	// and on the same network (same VLAN ID/VNI and same CIDR).
	mc.checkSubnetConsistency(subnetAttrs, &result)

	return result
}

func (mc *metroCheck) getMetro(ctx context.Context, result *preflight.CheckResult) (*capxv1.NutanixMetro, bool) {
	metroObj := &capxv1.NutanixMetro{}
	metroKey := ctrlclient.ObjectKey{Name: mc.metroName, Namespace: mc.namespace}
	if err := mc.kclient.Get(ctx, metroKey, metroObj); err != nil {
		if errors.IsNotFound(err) {
			failCheck(result, mc.field, fmt.Sprintf(
				"NutanixMetro %q was not found in the management cluster. Please create it and retry.",
				mc.metroName,
			))
			return nil, false
		}
		failCheckInternal(result, mc.field, fmt.Sprintf(
			"Failed to get NutanixMetro %q: %s. This is usually a temporary error. Please retry.",
			mc.metroName,
			err,
		))
		return nil, false
	}
	return metroObj, true
}

func (mc *metroCheck) evaluateFailureDomains(
	ctx context.Context,
	metroObj *capxv1.NutanixMetro,
	subnetAttrs *metroSubnetAttributes,
	result *preflight.CheckResult,
) map[string]struct{} {
	peUUIDs := map[string]struct{}{}
	for _, fdRef := range metroObj.Spec.FailureDomains {
		if fdRef.Name == "" {
			continue
		}

		fdObj, ok := mc.getFailureDomain(ctx, fdRef.Name, result)
		if !ok {
			continue
		}

		peCluster, ok := mc.resolvePrismElement(ctx, fdObj, fdRef.Name, result)
		if !ok {
			continue
		}
		peUUID := *peCluster.ExtId
		peUUIDs[peUUID] = struct{}{}

		if !mc.collectSubnetAttributes(ctx, fdObj, fdRef.Name, peUUID, subnetAttrs, result) {
			continue
		}
	}

	return peUUIDs
}

func (mc *metroCheck) getFailureDomain(
	ctx context.Context,
	fdName string,
	result *preflight.CheckResult,
) (*capxv1.NutanixFailureDomain, bool) {
	fdObj := &capxv1.NutanixFailureDomain{}
	fdKey := ctrlclient.ObjectKey{Name: fdName, Namespace: mc.namespace}
	if err := mc.kclient.Get(ctx, fdKey, fdObj); err != nil {
		if errors.IsNotFound(err) {
			failCheck(result, mc.field, fmt.Sprintf(
				"NutanixFailureDomain %q referenced by NutanixMetro %q was not found in the management cluster. Please create it and retry.", //nolint:lll // Message is long.
				fdName,
				mc.metroName,
			))
			return nil, false
		}
		failCheckInternal(result, mc.field, fmt.Sprintf(
			"Failed to get NutanixFailureDomain %q referenced by NutanixMetro %q: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
			fdName,
			mc.metroName,
			err,
		))
		return nil, false
	}

	return fdObj, true
}

// resolvePrismElement resolves the single Prism Element cluster for a failure
// domain. It appends a cause and returns ok=false when resolution fails.
func (mc *metroCheck) resolvePrismElement(
	ctx context.Context,
	fdObj *capxv1.NutanixFailureDomain,
	fdName string,
	result *preflight.CheckResult,
) (*clustermgmtv4.Cluster, bool) {
	peIdentifier := fdObj.Spec.PrismElementCluster
	peClusters, err := getClusters(ctx, mc.nclient, &peIdentifier)
	if err != nil {
		failCheckInternal(result, mc.field, fmt.Sprintf(
			"Failed to check the Prism Element cluster %q referenced by Failure Domain %q of NutanixMetro %q: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
			peIdentifier,
			fdName,
			mc.metroName,
			err,
		))
		return nil, false
	}
	if len(peClusters) != 1 {
		failCheck(result, mc.field, fmt.Sprintf(
			"Found %d Prism Element cluster(s) that match identifier %q referenced by Failure Domain %q of NutanixMetro %q. There must be exactly 1 Cluster that matches this identifier.", //nolint:lll // Message is long.
			len(peClusters),
			peIdentifier,
			fdName,
			mc.metroName,
		))
		return nil, false
	}
	if peClusters[0].ExtId == nil {
		failCheckInternal(result, mc.field, fmt.Sprintf(
			"The Prism Element cluster %q referenced by Failure Domain %q of NutanixMetro %q was returned without an ExtId. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
			peIdentifier,
			fdName,
			mc.metroName,
		))
		return nil, false
	}
	return &peClusters[0], true
}

// metroSubnetAttributes accumulates the distinct network attributes observed
// across all subnets of a NutanixMetro's failure domains.
type metroSubnetAttributes struct {
	// layers holds the distinct network layer names (VLAN, OVERLAY, ...).
	layers map[string]struct{}
	// profilesByFailureDomain holds subnet profile sets keyed by failure domain.
	profilesByFailureDomain map[string]map[string]struct{}
}

func newMetroSubnetAttributes() *metroSubnetAttributes {
	return &metroSubnetAttributes{
		layers:                  map[string]struct{}{},
		profilesByFailureDomain: map[string]map[string]struct{}{},
	}
}

// collectSubnetAttributes records the network attributes of each subnet
// referenced by a failure domain into attrs. It appends a cause and returns
// false on a non-recoverable error.
func (mc *metroCheck) collectSubnetAttributes(
	ctx context.Context,
	fdObj *capxv1.NutanixFailureDomain,
	fdName string,
	peUUID string,
	attrs *metroSubnetAttributes,
	result *preflight.CheckResult,
) bool {
	for _, id := range fdObj.Spec.Subnets {
		subnets, err := getSubnets(ctx, mc.nclient, &id)
		if err != nil {
			failCheckInternal(result, mc.field, fmt.Sprintf(
				"Failed to get subnet %q referenced by Failure Domain %q of NutanixMetro %q: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
				id,
				fdName,
				mc.metroName,
				err,
			))
			return false
		}

		// Valid subnets either belong to this failure domain's PE, or are overlay
		// subnets with no cluster reference.
		for i := range subnets {
			s := &subnets[i]
			if s.ClusterReference != nil && *s.ClusterReference != peUUID {
				continue
			}
			profile := subnetProfileKey(s)
			attrs.layers[subnetLayerName(s.SubnetType)] = struct{}{}
			if _, ok := attrs.profilesByFailureDomain[fdName]; !ok {
				attrs.profilesByFailureDomain[fdName] = map[string]struct{}{}
			}
			attrs.profilesByFailureDomain[fdName][profile] = struct{}{}
		}
	}
	return true
}

// checkSubnetConsistency verifies that all observed subnets reside on the same
// supported network layer (VLAN-tagged or Overlay), and that every failure
// domain has the same set of subnet profiles (layer + VLAN/VNI + CIDR).
func (mc *metroCheck) checkSubnetConsistency(attrs *metroSubnetAttributes, result *preflight.CheckResult) {
	if len(attrs.layers) == 0 {
		return
	}

	if len(attrs.layers) > 1 {
		failCheck(result, mc.field, fmt.Sprintf(
			"Subnets across the failure domains of NutanixMetro %q reside on multiple network layers (%s). All subnets must reside on the same network layer (either VLAN-tagged or Overlay) to support synchronous metro replication.", //nolint:lll // Message is long.
			mc.metroName,
			strings.Join(sortedKeys(attrs.layers), ", "),
		))
		return
	}

	layer := sortedKeys(attrs.layers)[0]
	if layer != subnetLayerName(netv4.SUBNETTYPE_VLAN.Ref()) &&
		layer != subnetLayerName(netv4.SUBNETTYPE_OVERLAY.Ref()) {
		failCheck(result, mc.field, fmt.Sprintf(
			"Subnets of NutanixMetro %q reside on an unsupported network layer %q. Metro configurations support only VLAN-tagged or Overlay subnets.", //nolint:lll // Message is long.
			mc.metroName,
			layer,
		))
		return
	}

	if !equalSubnetProfileSets(attrs.profilesByFailureDomain) {
		failCheck(result, mc.field, fmt.Sprintf(
			"Subnets across the failure domains of NutanixMetro %q do not match. Each failure domain must expose the same set of subnet profiles (layer, VLAN ID/VNI, CIDR) to support synchronous metro replication.", //nolint:lll // Message is long.
			mc.metroName,
		))
	}
}

func subnetProfileKey(s *netv4.Subnet) string {
	layer := subnetLayerName(s.SubnetType)
	vlanID := ""
	cidr := ""
	if s.NetworkId != nil {
		vlanID = strconv.Itoa(*s.NetworkId)
	}
	if s.IpPrefix != nil {
		cidr = *s.IpPrefix
	}
	return fmt.Sprintf("%s|%s|%s", layer, vlanID, cidr)
}

func equalSubnetProfileSets(profilesByFD map[string]map[string]struct{}) bool {
	var baseline map[string]struct{}
	for _, profiles := range profilesByFD {
		if baseline == nil {
			baseline = profiles
			continue
		}
		if !stringSetEqual(baseline, profiles) {
			return false
		}
	}
	return true
}

func stringSetEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// subnetLayerName returns a stable, human-readable network layer name for a
// subnet type, treating a nil type as UNKNOWN.
func subnetLayerName(t *netv4.SubnetType) string {
	if t == nil {
		return "UNKNOWN"
	}
	return t.GetName()
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
