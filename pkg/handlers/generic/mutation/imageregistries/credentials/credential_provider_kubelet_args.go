// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

func addImageCredentialProviderArgs(args map[string]string) {
	args["image-credential-provider-bin-dir"] = credentialProviderTargetDir
	args["image-credential-provider-config"] = kubeletImageCredentialProviderConfigOnRemote
}
