// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"

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
			Allowed:  true,
			Warnings: []string{fmt.Sprintf("%s uses imageLookup, which is not yet supported by checks", c.field)},
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
						Message: fmt.Sprintf("failed to get VM Image: %s", err),
						Field:   c.field + ".image",
					},
				},
			}
		}

		if len(images) == 0 {
			return preflight.CheckResult{
				Allowed: true,
			}
		}

		if err := c.checkKubernetesVersion(&images[0]); err != nil {
			return preflight.CheckResult{
				Allowed:       false,
				InternalError: false,
				Causes: []preflight.Cause{
					{
						Message: err.Error(),
						Field:   c.field + ".image",
					},
				},
			}
		}
	}

	return preflight.CheckResult{Allowed: true}
}

func (c *imageKubernetesVersionCheck) checkKubernetesVersion(image *vmmv4.Image) error {
	imageName := ""
	if image.Name != nil {
		imageName = *image.Name
	}

	if imageName == "" {
		return fmt.Errorf("VM image name is empty")
	}

	parsedVersion, err := semver.Parse(c.clusterK8sVersion)
	if err != nil {
		return fmt.Errorf("failed to parse kubernetes version '%s': %v", c.clusterK8sVersion, err)
	}

	// For example, "1.33.1+fips.0" becomes "1.33.1".
	k8sVersion := parsedVersion.FinalizeVersion()

	if !strings.Contains(imageName, k8sVersion) {
		return fmt.Errorf(
			"cluster kubernetes version '%s' is not part of image name '%s'",
			c.clusterK8sVersion,
			imageName,
		)
	}

	return nil
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
				machineDetails: &cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails,
				field: "$.spec.topology.variables[?@.name==\"clusterConfig\"]." +
					".value.nutanix.controlPlane.machineDetails",
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
					field: fmt.Sprintf("$.spec.topology.workers.machineDeployments[?@.name==%q]"+
						".variables[?@.name=workerConfig].value.nutanix.machineDetails", mdName),
					nclient:           cd.nclient,
					clusterK8sVersion: clusterK8sVersion,
				},
			)
		}
	}

	return checks
}
