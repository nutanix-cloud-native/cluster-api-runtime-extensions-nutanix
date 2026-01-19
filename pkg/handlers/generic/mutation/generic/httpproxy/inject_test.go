// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestHTTPProxyPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "HTTP Proxy mutator suite")
}

var _ = Describe("Generate HTTPProxy Patches", func() {
	// only add HTTPProxy patch
	patchGenerator := func() mutation.GeneratePatches {
		// Always initialize the testEnv variable in the closure.
		// This will allow ginkgo to initialize testEnv variable during test execution time.
		clientScheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
		utilruntime.Must(clusterv1.AddToScheme(clientScheme))
		cl, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		gomega.Expect(err).To(gomega.BeNil())
		return mutation.NewMetaGeneratePatchesHandler(
			"",
			cl,
			NewPatch(cl)).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "http proxy set for KubeadmConfigTemplate generic worker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.HTTPProxy{
						HTTP:         "http://example.com",
						HTTPS:        "https://example.com",
						AdditionalNo: []string{"no-proxy.example.com"},
					},
					VariableName,
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					map[string]any{
						"machineDeployment": map[string]any{
							"class": names.SimpleNameGenerator.GenerateName("worker-"),
						},
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ContainElements(
					gomega.HaveKeyWithValue(
						"path", "/etc/systemd/system/containerd.service.d/http-proxy.conf",
					),
					gomega.HaveKeyWithValue(
						"path", "/etc/systemd/system/kubelet.service.d/http-proxy.conf",
					),
				),
			}},
		},
		{
			Name: "http proxy set for KubeadmControlPlaneTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.HTTPProxy{
						HTTP:         "http://example.com",
						HTTPS:        "https://example.com",
						AdditionalNo: []string{"no-proxy.example.com"},
					},
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElements(
					gomega.HaveKeyWithValue(
						"path", "/etc/systemd/system/containerd.service.d/http-proxy.conf",
					),
					gomega.HaveKeyWithValue(
						"path", "/etc/systemd/system/kubelet.service.d/http-proxy.conf",
					),
				),
			}},
		},
	}
	// create test node for each case
	for _, tt := range testDefs {
		It(tt.Name, func() {
			clientScheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
			utilruntime.Must(clusterv1.AddToScheme(clientScheme))
			cl, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			gomega.Expect(err).To(gomega.BeNil())
			c := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      request.ClusterName,
					Namespace: request.Namespace,
				},
			}
			err = cl.Create(context.Background(), c)
			gomega.Expect(err).To(gomega.BeNil())
			capitest.AssertGeneratePatches(
				GinkgoT(),
				patchGenerator,
				&tt,
			)
			err = cl.Delete(context.Background(), c)
			gomega.Expect(err).To(gomega.BeNil())
		})
	}
})
