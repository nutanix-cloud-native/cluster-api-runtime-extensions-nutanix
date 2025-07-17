// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type failureDomainCheck struct {
	failureDomainName string
	namespace         string
	field             string
	kclient           ctrlclient.Client
	nclient           client
}

func (fdc *failureDomainCheck) Name() string {
	return "NutanixFailureDomain"
}

func newFailureDomainChecks(cd *checkDependencies) []preflight.Check {
	checks := []preflight.Check{}

	if cd.nclient == nil || cd.kclient == nil {
		return checks
	}

	// For the failure domains configured for control-plane nodes
	if cd.nutanixClusterConfigSpec != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		for _, fdName := range cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.FailureDomains {
			if fdName != "" {
				checks = append(checks, &failureDomainCheck{
					failureDomainName: fdName,
					namespace:         cd.cluster.Namespace,
					field:             "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.failureDomains", //nolint:lll // field is long.
					kclient:           cd.kclient,
					nclient:           cd.nclient,
				})
			}
		}
	}

	// For the failure domains configured for worker nodes
	if cd.cluster != nil &&
		cd.cluster.Spec.Topology != nil &&
		cd.cluster.Spec.Topology.Workers != nil {
		for i := range cd.cluster.Spec.Topology.Workers.MachineDeployments {
			md := &cd.cluster.Spec.Topology.Workers.MachineDeployments[i]
			if md.FailureDomain != nil && *md.FailureDomain != "" {
				checks = append(checks, &failureDomainCheck{
					failureDomainName: *md.FailureDomain,
					namespace:         cd.cluster.Namespace,
					field: fmt.Sprintf(
						"$.spec.topology.workers.machineDeployments[?@.name==%q].failureDomain",
						md.Name,
					),
					kclient: cd.kclient,
					nclient: cd.nclient,
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

	// Fetch the referent failure domain object
	fdObj := &capxv1.NutanixFailureDomain{}
	fdKey := ctrlclient.ObjectKey{Name: fdc.failureDomainName, Namespace: fdc.namespace}
	if err := fdc.kclient.Get(ctx, fdKey, fdObj); err != nil {
		if errors.IsNotFound(err) {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"NutanixFailureDomain %q referenced in cluster was not found in the management cluster. Please create it and retry.", //nolint:lll // Message is long.
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
				"Failed to get NutanixFailureDomain %q: %v This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
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
	peClusters, err := getClusters(fdc.nclient, &peIdentifier)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to check if the Prism Element cluster %q, referenced by Failure Domain %q, exists: %v This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
				peIdentifier,
				fdc.failureDomainName,
				err,
			),
			Field: fdc.field,
		})
		return result
	}
	if len(peClusters) != 1 {
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
	peUUID := *peClusters[0].ExtId

	// Validate spec.subnets configuration
	for _, id := range fdObj.Spec.Subnets {
		subnets, err := getSubnets(fdc.nclient, &id)
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to get subnet %q referenced by the Failure Domain %q: %v This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
					id,
					fdc.failureDomainName,
					err,
				),
				Field: fdc.field,
			})
			continue
		}

		// Filter the subnets, the valid ones should either have clusterReference match peUUID
		// or clusterReference being nil (for overlay subnets)
		filteredSubnets := []netv4.Subnet{}
		for i := range subnets {
			if subnets[i].ClusterReference == nil || *subnets[i].ClusterReference == peUUID {
				filteredSubnets = append(filteredSubnets, subnets[i])
			}
		}

		if len(filteredSubnets) != 1 {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Found %d Subnets that match identifier %q. There must be exactly 1 Subnet that matches this identifier.", //nolint:lll // Message is long.,
					len(filteredSubnets),
					id,
				),
				Field: fdc.field,
			})
			continue
		}
	}

	return result
}

// getSubnets returns the subnets found in PC with the input identifier.
func getSubnets(client client, subnetId *capxv1.NutanixResourceIdentifier) ([]netv4.Subnet, error) {
	switch {
	case subnetId.IsUUID():
		resp, err := client.GetSubnetById(subnetId.UUID)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			// No subnet returned.
			return []netv4.Subnet{}, nil
		}
		subnet, ok := resp.GetData().(netv4.Subnet)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by GetSubnetById")
		}
		return []netv4.Subnet{subnet}, nil
	case subnetId.IsName():
		filter_ := fmt.Sprintf("name eq '%s'", *subnetId.Name)
		resp, err := client.ListSubnets(nil, nil, &filter_, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.GetData() == nil {
			// No subnets returned.
			return []netv4.Subnet{}, nil
		}
		subnets, ok := resp.GetData().([]netv4.Subnet)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by ListSubnets")
		}
		return subnets, nil
	default:
		return nil, fmt.Errorf("subnet identifier is missing both name and uuid, identifier type: %s", subnetId.Type)
	}
}
