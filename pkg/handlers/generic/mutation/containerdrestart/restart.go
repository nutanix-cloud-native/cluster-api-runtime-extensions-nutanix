// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package containerdrestart

import (
	_ "embed"

	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

const (
	ContainerdRestartScriptOnRemote        = "/etc/containerd/restart.sh"
	ContainerdRestartScriptOnRemoteCommand = "/bin/bash " + ContainerdRestartScriptOnRemote
)

//go:embed templates/containerd-restart.sh
var containerdRestartScript []byte

//nolint:gocritic // no need for named return values
func generateContainerdRestartScript() (bootstrapv1.File, string) {
	return bootstrapv1.File{
			Path:        ContainerdRestartScriptOnRemote,
			Content:     string(containerdRestartScript),
			Permissions: "0700",
		},
		ContainerdRestartScriptOnRemoteCommand
}
