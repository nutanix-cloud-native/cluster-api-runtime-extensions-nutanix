// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestExtraAPIServerCertSANsPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Extra API server certificate mutator suite")
}

type testObj struct {
	patchTest capitest.PatchTestDef
	cluster   clusterv1.Cluster
}

var _ = Describe("Generate Extra API server certificate patches", func() {
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
				Name: "extra API server cert SANs set with AWS",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						clusterconfig.MetaVariableName,
						v1alpha1.ClusterConfigSpec{
							GenericClusterConfig: v1alpha1.GenericClusterConfig{
								ExtraAPIServerCertSANs: v1alpha1.ExtraAPIServerCertSANs{
									"a.b.c.example.com",
									"d.e.f.example.com",
								},
							},
							AWS: &v1alpha1.AWSSpec{},
						},
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"certSANs",
							[]interface{}{"a.b.c.example.com", "d.e.f.example.com"},
						),
					),
				}},
			},
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: metav1.NamespaceDefault,
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "aws",
					},
				},
			},
		},
		{
			patchTest: capitest.PatchTestDef{
				Name: "extra API server cert SANs set with Docker",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						clusterconfig.MetaVariableName,
						v1alpha1.ClusterConfigSpec{
							GenericClusterConfig: v1alpha1.GenericClusterConfig{
								ExtraAPIServerCertSANs: v1alpha1.ExtraAPIServerCertSANs{
									"a.b.c.example.com",
								},
							},
						},
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"certSANs",
							[]interface{}{
								"a.b.c.example.com",
								"localhost",
								"127.0.0.1",
								"0.0.0.0",
								"host.docker.internal",
							},
						),
					),
				}},
			},
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: metav1.NamespaceDefault,
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "docker",
					},
				},
			},
		},
	}

	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.patchTest.Name, func() {
			clientScheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
			utilruntime.Must(clusterv1.AddToScheme(clientScheme))
			cl, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			gomega.Expect(err).To(gomega.BeNil())
			err = cl.Create(context.Background(), &tt.cluster)
			gomega.Expect(err).To(gomega.BeNil())
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt.patchTest)
			err = cl.Delete(context.Background(), &tt.cluster)
			gomega.Expect(err).To(gomega.BeNil())
		})
	}
})
