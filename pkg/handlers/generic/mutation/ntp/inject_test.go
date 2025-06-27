// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ntp

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestNTPPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "NTP mutator suite")
}

var _ = Describe("Generate NTP patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name:        "NTP configuration is set for control plane nodes with multiple servers",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/ntp",
					ValueMatcher: gomega.SatisfyAll(
						gomega.HaveKeyWithValue("enabled", gomega.BeTrue()),
						gomega.HaveKeyWithValue("servers", gomega.ConsistOf("time.aws.com", "time.google.com")),
					),
				},
			},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NTP{
						Servers: []string{"time.aws.com", "time.google.com"},
					},
					VariableName,
				),
			},
		},
		{
			Name:        "NTP configuration is set for control plane nodes with single server",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/ntp",
					ValueMatcher: gomega.SatisfyAll(
						gomega.HaveKeyWithValue("enabled", gomega.BeTrue()),
						gomega.HaveKeyWithValue("servers", gomega.ConsistOf("pool.ntp.org")),
					),
				},
			},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NTP{
						Servers: []string{"pool.ntp.org"},
					},
					VariableName,
				),
			},
		},
		{
			Name:        "NTP configuration is set for worker nodes with multiple servers",
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/ntp",
					ValueMatcher: gomega.SatisfyAll(
						gomega.HaveKeyWithValue("enabled", gomega.BeTrue()),
						gomega.HaveKeyWithValue(
							"servers",
							gomega.ConsistOf("0.pool.ntp.org", "1.pool.ntp.org", "2.pool.ntp.org"),
						),
					),
				},
			},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NTP{
						Servers: []string{"0.pool.ntp.org", "1.pool.ntp.org", "2.pool.ntp.org"},
					},
					VariableName,
				),
				capitest.VariableWithValue(
					"builtin",
					map[string]any{
						"machineDeployment": map[string]any{
							"class": "worker-class",
						},
					},
				),
			},
		},
		{
			Name:        "NTP configuration is set for worker nodes with single server",
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/ntp",
					ValueMatcher: gomega.SatisfyAll(
						gomega.HaveKeyWithValue("enabled", gomega.BeTrue()),
						gomega.HaveKeyWithValue("servers", gomega.ConsistOf("pool.ntp.org")),
					),
				},
			},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NTP{
						Servers: []string{"pool.ntp.org"},
					},
					VariableName,
				),
				capitest.VariableWithValue(
					"builtin",
					map[string]any{
						"machineDeployment": map[string]any{
							"class": "worker-class",
						},
					},
				),
			},
		},
		{
			Name:                  "NTP variable not set results in no patches",
			RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{},
		},
		{
			Name:                  "NTP servers empty results in no patches",
			RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NTP{
						Servers: []string{},
					},
					VariableName,
				),
			},
		},
	}

	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})
