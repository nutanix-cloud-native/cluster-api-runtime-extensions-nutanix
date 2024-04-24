// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanevirtualip

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestControlPlaneEndpointPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane virtual IP suite")
}

var _ = Describe("Generate ControlPlane virtual IP patches", func() {
	testDefs := []struct {
		capitest.PatchTestDef
		virtualIPTemplate string
	}{
		{
			PatchTestDef: capitest.PatchTestDef{
				Name: "host and port should be templated in a new file",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						clusterconfig.MetaVariableName,
						v1alpha1.ControlPlaneEndpointSpec{
							Host:          "10.20.100.10",
							Port:          6443,
							VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{},
						},
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
	for _, tt := range testDefs {
		It(tt.Name, func() {
			// Always initialize the testEnv variable in the closure.
			// This will allow ginkgo to initialize testEnv variable during test execution time.
			testEnv := helpers.TestEnv
			// use direct client instead of controller client. This will allow the patch handler to read k8s object
			// that are written by the tests.
			// Test cases writes credentials secret that the mutator handler reads.
			// Using direct client will enable reading it immediately.
			client, err := testEnv.GetK8sClient()
			gomega.Expect(err).To(gomega.BeNil())

			// setup a test ConfigMap to be used by the handler
			cm := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    corev1.NamespaceDefault,
					GenerateName: "virtualip-test-",
				},
				Data: map[string]string{
					"data": tt.virtualIPTemplate,
				},
			}
			err = client.Create(context.Background(), cm)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			cfg := &Config{
				GlobalOptions:               options.NewGlobalOptions(),
				defaultKubeVipConfigMapName: cm.Name,
			}
			patchGenerator := func() mutation.GeneratePatches {
				return mutation.NewMetaGeneratePatchesHandler(
					"",
					client,
					NewControlPlaneVirtualIP(client, cfg, clusterconfig.MetaVariableName, VariableName),
				).(mutation.GeneratePatches)
			}

			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt.PatchTestDef)
		})
	}
})

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
