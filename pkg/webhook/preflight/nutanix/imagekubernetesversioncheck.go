// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type imageKubernetesVersionCheck struct {
	machineDetails    *carenv1.NutanixMachineDetails
	field             string
	nclient           client
	clusterK8sVersion string
}

func (c *imageKubernetesVersionCheck) Name() string {
	return "NutanixVMImageKubernetesVersion"
}

func (c *imageKubernetesVersionCheck) Run(ctx context.Context) preflight.CheckResult {
	if c.machineDetails.ImageLookup != nil {
		return preflight.CheckResult{
			Allowed: true,
			Warnings: []string{
				fmt.Sprintf(
					"%s uses imageLookup, which is not yet supported by checks",
					c.field,
				),
			},
		}
	}

	if c.machineDetails.Image != nil {
		images, err := getVMImages(c.nclient, c.machineDetails.Image)
		if err != nil {
			return preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: fmt.Sprintf(
							"Failed to get VM Image %q: %s. This is usually a temporary error. Please retry.",
							c.machineDetails.Image,
							err,
						),
						Field: c.field + ".image",
					},
				},
			}
		}

		if len(images) == 0 {
			return preflight.CheckResult{
				Allowed: true,
			}
		}
		image := images[0]

		if image.Name == nil || *image.Name == "" {
			return preflight.CheckResult{
				Allowed:       false,
				InternalError: false,
				Causes: []preflight.Cause{
					{
						Message: fmt.Sprintf(
							"The VM Image identified by %q has no name. Give the VM Image a name, or use a different VM Image. Make sure the VM Image contains the Kubernetes version supported by the VM Image. Choose a VM Image that supports the cluster Kubernetes version: %q", //nolint:lll // The message is long.
							*c.machineDetails.Image,
							c.clusterK8sVersion,
						),
						Field: c.field + ".image",
					},
				},
			}
		}

		// Uses the same function that is used by the Cluster API topology validation webhook.
		parsedClusterK8sVersion, err := semver.ParseTolerant(c.clusterK8sVersion)
		if err != nil {
			return preflight.CheckResult{
				Allowed: false,
				// The Cluster API topology validation webhook should prevent this from happening,
				// so if it does, treat it as an internal error.
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: fmt.Sprintf(
							"The Cluster Kubernetes version %q is not a valid semantic version. This error should not happen under normal circumstances. Please report.", //nolint:lll // The message is long.
							c.clusterK8sVersion,
						),
						Field: c.field + ".image",
					},
				},
			}
		}

		finalizedClusterK8sVersion := parsedClusterK8sVersion.FinalizeVersion()
		if !strings.Contains(*image.Name, finalizedClusterK8sVersion) {
			return preflight.CheckResult{
				Allowed:       false,
				InternalError: false,
				Causes: []preflight.Cause{
					{
						Message: fmt.Sprintf(
							"The VM Image identified by %q has the name %q. Make sure the VM Image name contains the Kubernetes version supported by the VM Image. Choose a VM Image that supports the cluster Kubernetes version: %q.", //nolint:lll // The message is long.
							*c.machineDetails.Image,
							*image.Name,
							finalizedClusterK8sVersion,
						),
						Field: c.field + ".image",
					},
				},
			}
		}
	}

	return preflight.CheckResult{Allowed: true}
}

func newVMImageKubernetesVersionChecks(
	cd *checkDependencies,
) []preflight.Check {
	checks := make([]preflight.Check, 0)

	if cd.nclient == nil {
		return checks
	}

	// Get cluster Kubernetes version for version matching
	clusterK8sVersion := ""
	if cd.cluster != nil && cd.cluster.Spec.Topology != nil && cd.cluster.Spec.Topology.Version != "" {
		clusterK8sVersion = strings.TrimPrefix(cd.cluster.Spec.Topology.Version, "v")
	}

	// If cluster Kubernetes version is not specified, skip the check.
	if clusterK8sVersion == "" {
		return checks
	}

	if cd.nutanixClusterConfigSpec != nil && cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		checks = append(checks,
			&imageKubernetesVersionCheck{
				machineDetails:    &cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails,
				field:             "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.nutanix.controlPlane.machineDetails", ///nolint:lll // Field is long.
				nclient:           cd.nclient,
				clusterK8sVersion: clusterK8sVersion,
			},
		)
	}

	for mdName, nutanixWorkerNodeConfigSpec := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if nutanixWorkerNodeConfigSpec.Nutanix != nil {
			checks = append(checks,
				&imageKubernetesVersionCheck{
					machineDetails: &nutanixWorkerNodeConfigSpec.Nutanix.MachineDetails,
					//nolint:lll // The field is long.
					field: fmt.Sprintf(
						"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=workerConfig].value.nutanix.machineDetails",
						mdName,
					),
					nclient:           cd.nclient,
					clusterK8sVersion: clusterK8sVersion,
				},
			)
		}
	}

	return checks
}
