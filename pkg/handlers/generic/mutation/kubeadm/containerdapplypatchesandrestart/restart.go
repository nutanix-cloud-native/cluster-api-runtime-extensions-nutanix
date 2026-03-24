// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package containerdapplypatchesandrestart

import (
	_ "embed"

	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common"
)

var (
	containerdRestartScriptOnRemote        = common.ContainerdScriptPathOnRemote("restart.sh")
	containerdRestartScriptOnRemoteCommand = "/bin/bash " + containerdRestartScriptOnRemote
)

//go:embed templates/containerd-restart.sh
var containerdRestartScript []byte

//nolint:gocritic // no need for named return values
func generateContainerdRestartScript() (bootstrapv1.File, string) {
	return bootstrapv1.File{
			Path:        containerdRestartScriptOnRemote,
			Content:     string(containerdRestartScript),
			Permissions: "0700",
		},
		containerdRestartScriptOnRemoteCommand
}
