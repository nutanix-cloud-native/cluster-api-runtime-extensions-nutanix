// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	_ "embed"

	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const (
	computeReservationsScriptPath = "/etc/caren/compute-kubelet-reservations.sh"
	computeReservationsCommand    = "/bin/sh " + computeReservationsScriptPath
)

//go:embed embedded/compute-reservations.sh
var computeReservationsScript string

func automaticReservationsEnabled(cfg *v1alpha1.KubeletConfiguration) bool {
	return cfg != nil && cfg.AutomaticReservations != nil
}

func computeReservationsScriptFile() bootstrapv1.File {
	return bootstrapv1.File{
		Path:        computeReservationsScriptPath,
		Owner:       "root:root",
		Permissions: "0755",
		Content:     computeReservationsScript,
	}
}
