// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"

	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
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
	errMessage *string
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
						field:             "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.failureDomains", //nolint:lll // field is long.
						kclient:           cd.kclient,
						nclient:           cd.nclient,
					}
					check.errMessage = ptr.To(err.Error())

					checks = append(checks, check)
					continue
				}

				for _, fdName := range fdNames {
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
					field:             "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.failureDomains", //nolint:lll // field is long.
					kclient:           cd.kclient,
					nclient:           cd.nclient,
				}
				check.errMessage = ptr.To(err.Error())

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
		subnets, err := getSubnets(ctx, fdc.nclient, &id)
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
		subnet, ok := resp.GetData().(netv4.Subnet)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by GetSubnetById")
		}
		return []netv4.Subnet{subnet}, nil
	case subnetId.IsName():
		filter_ := fmt.Sprintf("name eq '%s'", *subnetId.Name)
		resp, err := client.ListSubnets(ctx, nil, nil, &filter_, nil, nil, nil)
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
