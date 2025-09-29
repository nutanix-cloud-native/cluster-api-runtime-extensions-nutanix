// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nodelabels

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate node-labels patches for Worker", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewWorkerPatch()).(mutation.GeneratePatches)
	}

	// helper: build a KubeadmConfigTemplate request item with a specific initial node-labels value
	newKCTWithNodeLabels := func(nodeLabels string) runtimehooksv1.GeneratePatchesRequestItem {
		args := map[string]string{
			"cloud-provider": "external",
		}
		if nodeLabels != "" {
			args["node-labels"] = nodeLabels
		}

		return request.NewRequestItem(
			&bootstrapv1.KubeadmConfigTemplate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: bootstrapv1.GroupVersion.String(),
					Kind:       "KubeadmConfigTemplate",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kubeadmconfigtemplate",
					Namespace: request.Namespace,
				},
				Spec: bootstrapv1.KubeadmConfigTemplateSpec{
					Template: bootstrapv1.KubeadmConfigTemplateResource{
						Spec: bootstrapv1.KubeadmConfigSpec{
							JoinConfiguration: &bootstrapv1.JoinConfiguration{
								NodeRegistration: bootstrapv1.NodeRegistrationOptions{
									KubeletExtraArgs: args,
								},
							},
						},
					},
				},
			},
			&runtimehooksv1.HolderReference{
				Kind:      "MachineDeployment",
				FieldPath: "spec.template.spec.infrastructureRef",
			},
			types.UID(""),
		)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "adds worker role label when missing",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`)},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/joinConfiguration/nodeRegistration/kubeletExtraArgs/node-labels",
				ValueMatcher: gomega.Equal(
					"node-role.kubernetes.io/worker=",
				),
			}},
		},
		{
			Name: "no patch if worker role label already present",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`)},
				),
			},
			RequestItem: newKCTWithNodeLabels("node-role.kubernetes.io/worker="),
		},
		{
			Name: "merge worker role label with existing labels",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`)},
				),
			},
			RequestItem: newKCTWithNodeLabels("env=prod"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "replace",
				Path:      "/spec/template/spec/joinConfiguration/nodeRegistration/kubeletExtraArgs/node-labels",
				ValueMatcher: gomega.SatisfyAll(
					gomega.ContainSubstring("env=prod"),
					gomega.ContainSubstring("node-role.kubernetes.io/worker="),
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
