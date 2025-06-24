// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

// Examples: nkp-ubuntu-22.04-vgpu-1.32.3-20250604180644, nkp-rocky-9.5-release-cis-1.32.3-20250430150550.
// The regex captures the Kubernetes version in the format of 1.x.y, where x and y are digits.
var kubernetesVersionRegex = regexp.MustCompile(`(?i)\b[vV]?(1\.\d+(?:\.\d+)?)\b`)

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
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: fmt.Sprintf("failed to get VM Image: %s", err),
						Field:   c.field,
					},
				},
			}
		}

		if len(images) == 0 {
			return preflight.CheckResult{
				Allowed:  true,
				Warnings: []string{"expected to find 1 VM Image, found none"},
			}
		}

		if err := c.checkKubernetesVersion(&images[0]); err != nil {
			return preflight.CheckResult{
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: err.Error(),
						Field:   c.field,
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

	imageK8sVersion, err := extractKubernetesVersionFromImageName(imageName)
	if err != nil {
		return fmt.Errorf("failed to extract Kubernetes version from image name '%s': %s. "+
			"This check assumes a naming convention that includes kubernetes version in the name. "+
			"You can opt out of this check if using non-compliant naming", imageName, err)
	}

	if imageK8sVersion != c.clusterK8sVersion {
		return fmt.Errorf(
			"kubernetes version mismatch: cluster version '%s' does not match image version '%s' (from image name '%s')",
			c.clusterK8sVersion,
			imageK8sVersion,
			imageName,
		)
	}

	return nil
}

// extractKubernetesVersionFromImageName extracts the Kubernetes version from the given image name.
// It expects something that looks like a kubernetes version in the image name i.e. 1.x.y?,
// Examples: nkp-ubuntu-22.04-vgpu-1.32.3-20250604180644 -> 1.32.3.
func extractKubernetesVersionFromImageName(imageName string) (string, error) {
	matches := kubernetesVersionRegex.FindStringSubmatch(imageName)
	if len(matches) < 2 {
		return "", fmt.Errorf(
			"image name does not match expected naming convention (expected pattern: .*<k8s-version>.*)",
		)
	}
	return matches[1], nil
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
				field: "cluster.spec.topology.variables[.name=clusterConfig]" +
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
					field: fmt.Sprintf("cluster.spec.topology.workers.machineDeployments[.name=%s]"+
						".variables[.name=workerConfig].value.nutanix.machineDetails", mdName),
					nclient:           cd.nclient,
					clusterK8sVersion: clusterK8sVersion,
				},
			)
		}
	}

	return checks
}
