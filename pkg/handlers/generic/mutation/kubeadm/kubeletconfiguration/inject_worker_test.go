// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate KubeletConfiguration patches for Worker", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewWorkerPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "kubeletConfiguration set at cluster level is ignored",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.KubeletConfiguration{
						PodPidsLimit: ptr.To(int64(4096)),
					},
					VariableName,
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		},
		{
			Name: "kubeletConfiguration set at worker override",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.KubeletConfiguration{
						MaxPods: ptr.To(int32(110)),
					},
					VariableName,
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.And(
						gomega.HaveKeyWithValue(
							"path",
							kubeletConfigurationPatchFilePath,
						),
						gomega.HaveKeyWithValue(
							"content",
							`---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
maxPods: 110
`,
						),
					),
				),
			}},
		},
		{
			Name: "maxPods only does not inject reservation script",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.KubeletConfiguration{MaxPods: ptr.To(int32(110))},
					VariableName,
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.Not(gomega.ContainElement(
					gomega.HaveKeyWithValue("path", computeReservationsScriptPath),
				)),
			}},
		},
		{
			Name: "automaticReservations at worker injects script and preKubeadmCommand",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.KubeletConfiguration{
						AutomaticReservations: &v1alpha1.AutomaticReservations{
							Profile: v1alpha1.ReservationProfileCapacityTiered,
						},
					},
					VariableName,
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/files",
					ValueMatcher: gomega.ContainElement(
						gomega.HaveKeyWithValue("path", computeReservationsScriptPath),
					),
				},
				{
					Operation:    "add",
					Path:         "/spec/template/spec/preKubeadmCommands",
					ValueMatcher: gomega.ContainElement(computeReservationsCommand),
				},
			},
		},
		{
			Name: "seccompDefault set at worker is rendered into patch",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.KubeletConfiguration{
						SeccompDefault: ptr.To(true),
					},
					VariableName,
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.And(
						gomega.HaveKeyWithValue(
							"path",
							kubeletConfigurationPatchFilePath,
						),
						gomega.HaveKeyWithValue(
							"content",
							`---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
seccompDefault: true
`,
						),
					),
				),
			}},
		},
	}

	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(
				GinkgoT(),
				patchGenerator,
				&tt,
			)
		})
	}
})
