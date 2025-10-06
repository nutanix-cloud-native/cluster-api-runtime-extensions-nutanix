// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package coredns

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestKubernetesDNSPatchHandlerPatchHandler(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "CoreDNS mutator suite")
}

type testObj struct {
	patchTest capitest.PatchTestDef
	cluster   clusterv1.Cluster
}

var _ = Describe("Generate CoreDNS patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		clientScheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
		utilruntime.Must(clusterv1.AddToScheme(clientScheme))
		cl, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		gomega.Expect(err).To(gomega.BeNil())
		return mutation.NewMetaGeneratePatchesHandler("", cl, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []testObj{
		{
			patchTest: capitest.PatchTestDef{
				Name: "unset variable",
			},
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: request.Namespace,
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "nutanix",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "1.30.100",
					},
				},
			},
		},
		{
			patchTest: capitest.PatchTestDef{
				Name: "variable with defaults",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.CoreDNS{},
						v1alpha1.DNSVariableName,
					),
				},
			},
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: request.Namespace,
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "nutanix",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "1.30.100",
					},
				},
			},
		},
		{
			patchTest: capitest.PatchTestDef{
				Name: "variable with defaults",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.CoreDNS{},
						v1alpha1.DNSVariableName,
						VariableName,
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"dns",
						map[string]interface{}{
							"imageTag": "v1.11.3",
						},
					),
				}},
			},
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: request.Namespace,
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "nutanix",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "1.30.100",
					},
				},
			},
		},
		{
			patchTest: capitest.PatchTestDef{
				Name: "coreDNS imageRepository and imageTag set",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.CoreDNS{
							Image: &v1alpha1.Image{
								Repository: "my-registry.io/my-org/my-repo",
								Tag:        "v1.11.3_custom.0",
							},
						},
						v1alpha1.DNSVariableName,
						VariableName,
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"dns",
						map[string]interface{}{
							"imageRepository": "my-registry.io/my-org/my-repo",
							"imageTag":        "v1.11.3_custom.0",
						},
					),
				}},
			},
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: request.Namespace,
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "nutanix",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "1.30.100",
					},
				},
			},
		},
		{
			patchTest: capitest.PatchTestDef{
				Name: "coreDNS imageRepository set",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.CoreDNS{
							Image: &v1alpha1.Image{
								Repository: "my-registry.io/my-org/my-repo",
							},
						},
						v1alpha1.DNSVariableName,
						VariableName,
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"dns",
						map[string]interface{}{
							"imageRepository": "my-registry.io/my-org/my-repo",
							"imageTag":        "v1.11.3",
						},
					),
				}},
			},
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: request.Namespace,
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "nutanix",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "1.30.100",
					},
				},
			},
		},
		{
			patchTest: capitest.PatchTestDef{
				Name: "coreDNS imageTag set",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.CoreDNS{
							Image: &v1alpha1.Image{
								Tag: "v1.11.3_custom.0",
							},
						},
						v1alpha1.DNSVariableName,
						VariableName,
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"dns",
						map[string]interface{}{
							"imageTag": "v1.11.3_custom.0",
						},
					),
				}},
			},
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: request.Namespace,
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "nutanix",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "1.30.100",
					},
				},
			},
		},
		{
			patchTest: capitest.PatchTestDef{
				Name:        "error if cannot find default CoreDNS version",
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.CoreDNS{},
						v1alpha1.DNSVariableName,
						VariableName,
					),
				},
				ExpectedFailure: true,
			},
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: request.Namespace,
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "nutanix",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "1.100.100",
					},
				},
			},
		},
		{
			patchTest: capitest.PatchTestDef{
				Name: "no error if cannot find default CoreDNS version but variable is set",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.CoreDNS{
							Image: &v1alpha1.Image{
								Repository: "my-registry.io/my-org/my-repo",
								Tag:        "v1.11.3_custom.0",
							},
						},
						v1alpha1.DNSVariableName,
						VariableName,
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"dns",
						map[string]interface{}{
							"imageRepository": "my-registry.io/my-org/my-repo",
							"imageTag":        "v1.11.3_custom.0",
						},
					),
				}},
			},
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: request.Namespace,
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "nutanix",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "1.100.100",
					},
				},
			},
		},
	}

	// create test node for each case
	for _, tt := range testDefs {
		It(tt.patchTest.Name, func() {
			clientScheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
			utilruntime.Must(clusterv1.AddToScheme(clientScheme))
			cl, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			gomega.Expect(err).To(gomega.BeNil())
			err = cl.Create(context.Background(), &tt.cluster)
			gomega.Expect(err).To(gomega.BeNil())
			defer func() {
				err = cl.Delete(context.Background(), &tt.cluster)
				gomega.Expect(err).To(gomega.BeNil())
			}()

			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt.patchTest)
		})
	}
})
