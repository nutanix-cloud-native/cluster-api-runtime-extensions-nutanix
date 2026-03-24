// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
)

func addImageCredentialProviderArgs(args *[]bootstrapv1.Arg) {
	argsMap := make(map[string]bool)
	for _, arg := range *args {
		argsMap[arg.Name] = true
	}
	if !argsMap["image-credential-provider-bin-dir"] {
		*args = append(*args, bootstrapv1.Arg{
			Name:  "image-credential-provider-bin-dir",
			Value: ptr.To(credentialProviderTargetDir),
		})
	}
	if !argsMap["image-credential-provider-config"] {
		*args = append(*args, bootstrapv1.Arg{
			Name:  "image-credential-provider-config",
			Value: ptr.To(kubeletImageCredentialProviderConfigOnRemote),
		})
	}
}
