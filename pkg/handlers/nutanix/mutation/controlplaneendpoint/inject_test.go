// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneendpoint

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common/controlplaneendpoint/virtualip"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestControlPlaneEndpointPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Nutanix ControlPlane endpoint suite")
}

var _ = Describe("Generate Nutanix ControlPlane endpoint patches", func() {
	Context("testing NutanixClusterTemplate patches", func() {
		testDefs := []capitest.PatchTestDef{
			{
				Name: "unset variable",
			},
			{
				Name: "ControlPlaneEndpoint set to valid host and port",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						clusterconfig.MetaVariableName,
						v1alpha1.ControlPlaneEndpointSpec{
							Host: "10.20.100.10",
							Port: 6443,
						},
						nutanixclusterconfig.NutanixVariableName,
						VariableName,
					),
				},
				RequestItem: request.NewNutanixClusterTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
					{
						Operation:    "replace",
						Path:         "/spec/template/spec/controlPlaneEndpoint/host",
						ValueMatcher: gomega.Equal("10.20.100.10"),
					},
					{
						Operation:    "replace",
						Path:         "/spec/template/spec/controlPlaneEndpoint/port",
						ValueMatcher: gomega.BeEquivalentTo(6443),
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
					// nil works because these test cases won't trigger the KubeadmControlPlaneTemplate mutation
					testPatchGenerator(nil),
					&tt,
				)
			})
		}
	})

	Context("testing KubeadmControlPlaneTemplate patches", func() {
		testDefs := []struct {
			capitest.PatchTestDef
			virtualIPTemplate string
		}{
			{
				PatchTestDef: capitest.PatchTestDef{
					Name: "unset variable",
				},
			},
			{
				PatchTestDef: capitest.PatchTestDef{
					Name: "host and port should be templated in a new file",
					Vars: []runtimehooksv1.Variable{
						capitest.VariableWithValue(
							clusterconfig.MetaVariableName,
							v1alpha1.ControlPlaneEndpointSpec{
								Host: "10.20.100.10",
								Port: 6443,
							},
							nutanixclusterconfig.NutanixVariableName,
							VariableName,
						),
					},
					RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
					ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
						{
							Operation: "add",
							Path:      "/spec/template/spec/kubeadmConfigSpec/files",
							ValueMatcher: gomega.ContainElements(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue(
										"content",
										gomega.ContainSubstring("value: \"10.20.100.10\""),
									),
									gomega.HaveKeyWithValue(
										"content",
										gomega.ContainSubstring("value: \"6443\""),
									),
									gomega.HaveKey("owner"),
									gomega.HaveKeyWithValue("path", gomega.ContainSubstring("kube-vip")),
									gomega.HaveKey("permissions"),
								),
							),
						},
					},
				},
				virtualIPTemplate: validKubeVIPTemplate,
			},
		}

		// create test node for each case
		for testIdx := range testDefs {
			tt := testDefs[testIdx]
			It(tt.Name, func() {
				virtualIPProvider, err := virtualip.NewFromReaderProvider(strings.NewReader(tt.virtualIPTemplate))
				gomega.Expect(err).To(gomega.BeNil())

				capitest.AssertGeneratePatches(
					GinkgoT(),
					testPatchGenerator(virtualIPProvider),
					&tt.PatchTestDef,
				)
			})
		}
	})
})

func testPatchGenerator(virtualIPProvider virtualip.Provider) func() mutation.GeneratePatches {
	return func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler(
			"",
			helpers.TestEnv.Client,
			NewPatch().WithVirtualIPProvider(virtualIPProvider),
		).(mutation.GeneratePatches)
	}
}

var validKubeVIPTemplate = `
apiVersion: v1
kind: Pod
metadata:
  name: kube-vip
  namespace: kube-system
spec:
  containers:
    - name: kube-vip
      image: ghcr.io/kube-vip/kube-vip:v1.1.1
      imagePullPolicy: IfNotPresent
      args:
        - manager
      env:
        - name: vip_arp
          value: "true"
        - name: address
          value: "{{ .ControlPlaneEndpoint.Host }}"
        - name: port
          value: "{{ .ControlPlaneEndpoint.Port }}"
`
