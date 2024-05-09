// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package containerdapplypatchesandrestart

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestContainerdApplyPatchesAndRestartPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Containerd apply patches and restart mutator suite")
}

var _ = Describe("Generate Containerd apply patches and restart patches", func() {
	// only add aws region patch
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name:        "restart script and command added to control plane kubeadm config spec",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", containerdApplyPatchesScriptOnRemote,
						),
						gomega.HaveKeyWithValue(
							"path", ContainerdRestartScriptOnRemote,
						),
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
					ValueMatcher: gomega.HaveExactElements(
						containerdApplyPatchesScriptOnRemoteCommand,
						ContainerdRestartScriptOnRemoteCommand,
					),
				},
			},
		},
		{
			Name: "restart script and command added to worker node kubeadm config template",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					"builtin",
					map[string]any{
						"machineDeployment": map[string]any{
							"class": "*",
						},
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/files",
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", containerdApplyPatchesScriptOnRemote,
						),
						gomega.HaveKeyWithValue(
							"path", ContainerdRestartScriptOnRemote,
						),
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/preKubeadmCommands",
					ValueMatcher: gomega.HaveExactElements(
						containerdApplyPatchesScriptOnRemoteCommand,
						ContainerdRestartScriptOnRemoteCommand,
					),
				},
			},
		},
	}

	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(
				GinkgoT(),
				patchGenerator,
				&tt,
			)
		})
	}
})

func Test_generateContainerdApplyPatchesScript(t *testing.T) {
	wantFile := bootstrapv1.File{
		Path:        "/etc/containerd/apply-patches.sh",
		Owner:       "",
		Permissions: "0700",
		Encoding:    "",
		Append:      false,
		//nolint:lll // just a long string
		Content: `#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

declare -r TOML_MERGE_IMAGE="ghcr.io/mesosphere/toml-merge:v0.2.0"

if ! ctr --namespace k8s.io images check "name==${TOML_MERGE_IMAGE}" | grep "${TOML_MERGE_IMAGE}" >/dev/null; then
  ctr --namespace k8s.io images pull "${TOML_MERGE_IMAGE}"
fi

cleanup() {
  ctr images unmount "${tmp_ctr_mount_dir}" || true
}

trap 'cleanup' EXIT

readonly tmp_ctr_mount_dir="$(mktemp -d)"

ctr --namespace k8s.io images mount "${TOML_MERGE_IMAGE}" "${tmp_ctr_mount_dir}"
"${tmp_ctr_mount_dir}/usr/local/bin/toml-merge" -i --patch-file "/etc/containerd/cre.d/*.toml" /etc/containerd/config.toml
`,
	}
	wantCmd := "/bin/bash /etc/containerd/apply-patches.sh"
	file, cmd, _ := generateContainerdApplyPatchesScript()
	assert.Equal(t, wantFile, file)
	assert.Equal(t, wantCmd, cmd)
}
