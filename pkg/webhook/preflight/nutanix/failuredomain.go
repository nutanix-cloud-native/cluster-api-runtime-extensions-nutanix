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
	return "NutanixFailureDomains"
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
					field: fmt.Sprintf(
						"cluster.spec.topology.variables[.name=clusterConfig].value.controlPlane.nutanix.failureDomains[%s]",
						fdName,
					),
					kclient: cd.kclient,
					nclient: cd.nclient,
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
						"cluster.spec.topology.workers.machineDeployments[.name=%s].failureDomain[%s]",
						md.Name,
						*md.FailureDomain,
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
		msg := ""
		if errors.IsNotFound(err) {
			msg = fmt.Sprintf("not found the failure domain object with name %q", fdc.failureDomainName)
		} else {
			msg = fmt.Sprintf("failed to fetch the failure domain object with name %q: %v", fdc.failureDomainName, err)
		}

		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: msg,
			Field:   fdc.field,
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
				"failed to get the prism element cluster %s configured in the failure domain %q: %v",
				peIdentifier,
				fdc.failureDomainName,
				err,
			),
			Field: fmt.Sprintf("%s: .spec.prismElementCluster", fdc.field),
		})

		return result
	} else if len(peClusters) != 1 {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("expected to find 1 prism element cluster %s, found %d", peIdentifier, len(peClusters)),
			Field:   fmt.Sprintf("%s: .spec.prismElementCluster", fdc.field),
		})

		return result
	}
	peUUID := *peClusters[0].ExtId

	// Validate spec.subnets configuration
	for _, id := range fdObj.Spec.Subnets {
		subnets, err := getSubnets(fdc.nclient, &id, peUUID)
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"failed to get subnet %s configured in the failure domain %q: %v",
					id,
					fdc.failureDomainName,
					err,
				),
				Field: fmt.Sprintf("%s: .spec.subnets[%s]", fdc.field, id),
			})
			continue
		}

		if len(subnets) != 1 {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf("expected to find 1 subnet %s, found %d", id, len(subnets)),
				Field:   fmt.Sprintf("%s: .spec.subnets[%s]", fdc.field, id),
			})
			continue
		}
	}

	return result
}

// getSubnets returns the subnets found in PC with the input identifier and PE uuid.
func getSubnets(client client, subnetId *capxv1.NutanixResourceIdentifier, peUUID string) ([]netv4.Subnet, error) {
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
		filter_ := fmt.Sprintf("name eq '%s' and clusterReference eq '%s'", *subnetId.Name, peUUID)
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
