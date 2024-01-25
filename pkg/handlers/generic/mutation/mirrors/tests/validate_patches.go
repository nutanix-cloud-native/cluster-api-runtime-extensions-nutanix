// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/apiserver/pkg/storage/names"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/imageregistries"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/mirrors"
)

func TestValidatePatches(
	t *testing.T,
	generatorFunc func() mutation.GeneratePatches,
	variableName string,
) {
	t.Helper()

	capitest.ValidateGeneratePatches(
		t,
		generatorFunc,
		capitest.PatchTestDef{
			Name: "Different URLs for image registry and mirror registry is not allowed in KubeadmControlPlaneTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					variableWithRegistryAndMirror(
						"https://123456789.dkr.ecr.us-east-1.amazonaws.com",
						"https://987654321.dkr.ecr.us-west-2.amazonaws.com"),
				),
			},
			RequestItem:     request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedFailure: true,
		},
		capitest.PatchTestDef{
			Name: "Different URLs for image registry and mirror registry is not allowed in KubeadmConfigTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					variableWithRegistryAndMirror(
						"https://123456789.dkr.ecr.us-east-1.amazonaws.com",
						"https://987654321.dkr.ecr.us-west-2.amazonaws.com"),
				),
				capitest.VariableWithValue(
					"builtin",
					map[string]any{
						"machineDeployment": map[string]any{
							"class": names.SimpleNameGenerator.GenerateName("worker-"),
						},
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequest(
				"",
				workerRegistryAsMirrorCreds,
			),
			ExpectedFailure: true,
		},
		capitest.PatchTestDef{
			Name: "files added in KubeadmControlPlaneTemplate for registry and mirror with same URL",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					variableWithRegistryAndMirror(
						"https://12345678.dkr.ecr.us-east-1.amazonaws.com",
						"https://12345678.dkr.ecr.us-east-1.amazonaws.com"),
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.ContainElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/konvoy/install-kubelet-credential-providers.sh",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/kubernetes/image-credential-provider-config.yaml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/kubernetes/dynamic-credential-provider-config.yaml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
					ValueMatcher: gomega.ContainElement(
						"/bin/bash /etc/konvoy/install-kubelet-credential-providers.sh",
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/nodeRegistration/kubeletExtraArgs",
					ValueMatcher: gomega.HaveKeyWithValue(
						"image-credential-provider-bin-dir",
						"/etc/kubernetes/image-credential-provider/",
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/joinConfiguration/nodeRegistration/kubeletExtraArgs",
					ValueMatcher: gomega.HaveKeyWithValue(
						"image-credential-provider-config",
						"/etc/kubernetes/image-credential-provider-config.yaml",
					),
				},
			},
		},
		capitest.PatchTestDef{
			Name: "files added in KubeadmConfigTemplate for registry and mirror with same URL",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					variableWithRegistryAndMirror(
						"https://1234567.dkr.ecr.us-east-1.amazonaws.com",
						"https://1234567.dkr.ecr.us-east-1.amazonaws.com"),
				),
				capitest.VariableWithValue(
					"builtin",
					map[string]any{
						"machineDeployment": map[string]any{
							"class": names.SimpleNameGenerator.GenerateName("worker-"),
						},
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/files",
					ValueMatcher: gomega.ContainElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/konvoy/install-kubelet-credential-providers.sh",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/kubernetes/image-credential-provider-config.yaml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/kubernetes/dynamic-credential-provider-config.yaml",
						),
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/preKubeadmCommands",
					ValueMatcher: gomega.ContainElement(
						"/bin/bash /etc/konvoy/install-kubelet-credential-providers.sh",
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/joinConfiguration/nodeRegistration/kubeletExtraArgs",
					ValueMatcher: gomega.HaveKeyWithValue(
						"image-credential-provider-bin-dir",
						"/etc/kubernetes/image-credential-provider/",
					),
				},
			},
		},
	)
}

func variableWithRegistryAndMirror(registryURL, mirrorURL string) map[string]any {
	return map[string]any{
		imageregistries.VariableName: []v1alpha1.ImageRegistry{
			{
				URL: registryURL,
			},
		},
		mirrors.VariableName: v1alpha1.GlobalImageRegistryMirror{
			URL: mirrorURL,
		},
	}
}
