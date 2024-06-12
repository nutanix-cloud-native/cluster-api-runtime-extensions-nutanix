// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanevirtualip

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
		cluster           *clusterv1.Cluster
	}{
		{
			PatchTestDef: capitest.PatchTestDef{
				Name: "host and port should be templated in a new file and no pre/post commands",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.ControlPlaneEndpointSpec{
							Host: "10.20.100.10",
							Port: 6443,
							VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{
								Provider: v1alpha1.VirtualIPProviderKubeVIP,
							},
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
								gomega.HaveKeyWithValue(
									"path",
									gomega.ContainSubstring("kube-vip"),
								),
								gomega.HaveKey("permissions"),
							),
						),
					},
				},
				UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
						ValueMatcher: gomega.ContainElements(
							"/bin/bash /etc/caren/configure-for-kube-vip.sh set-host-aliases use-super-admin.conf",
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/postKubeadmCommands",
						ValueMatcher: gomega.ContainElements(
							"/bin/bash /etc/caren/configure-for-kube-vip.sh use-admin.conf",
						),
					},
				},
			},
			virtualIPTemplate: validKubeVIPTemplate,
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      request.ClusterName,
					Namespace: request.Namespace,
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Version: "v1.28.100",
					},
				},
			},
		},
		{
			PatchTestDef: capitest.PatchTestDef{
				Name: "host and port should be templated in a new file with pre/post commands",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.ControlPlaneEndpointSpec{
							Host: "10.20.100.10",
							Port: 6443,
							VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{
								Provider: v1alpha1.VirtualIPProviderKubeVIP,
							},
						},
						VariableName,
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(
					"",
				),
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
								gomega.HaveKeyWithValue(
									"path",
									gomega.ContainSubstring("kube-vip"),
								),
								gomega.HaveKey("permissions"),
							),
							gomega.SatisfyAll(
								gomega.HaveKey("content"),
								gomega.HaveKeyWithValue(
									"path",
									gomega.ContainSubstring("configure-for-kube-vip.sh"),
								),
								gomega.HaveKey("permissions"),
							),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
						ValueMatcher: gomega.ContainElements(
							"/bin/bash /etc/caren/configure-for-kube-vip.sh set-host-aliases use-super-admin.conf",
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/postKubeadmCommands",
						ValueMatcher: gomega.ContainElements(
							"/bin/bash /etc/caren/configure-for-kube-vip.sh use-admin.conf",
						),
					},
				},
			},
			virtualIPTemplate: validKubeVIPTemplate,
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      request.ClusterName,
					Namespace: request.Namespace,
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Version: "v1.29.0",
					},
				},
			},
		},
	}

	// create test node for each case
	for idx := range testDefs {
		tt := testDefs[idx]
		It(tt.Name, func() {
			clientScheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
			utilruntime.Must(clusterv1.AddToScheme(clientScheme))
			// Always initialize the testEnv variable in the closure.
			// This will allow ginkgo to initialize testEnv variable during test execution time.
			testEnv := helpers.TestEnv
			// use direct client instead of controller client. This will allow the patch handler to read k8s object
			// that are written by the tests.
			// Test cases writes credentials secret that the mutator handler reads.
			// Using direct client will enable reading it immediately.
			client, err := testEnv.GetK8sClientWithScheme(clientScheme)
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

			if tt.cluster != nil {
				err = client.Create(context.Background(), tt.cluster)
				gomega.Expect(err).To(gomega.BeNil())
				defer func() {
					err = client.Delete(context.Background(), tt.cluster)
					gomega.Expect(err).To(gomega.BeNil())
				}()
			}

			cfg := &Config{
				GlobalOptions:               options.NewGlobalOptions(),
				defaultKubeVIPConfigMapName: cm.Name,
			}
			patchGenerator := func() mutation.GeneratePatches {
				return mutation.NewMetaGeneratePatchesHandler(
					"",
					client,
					NewControlPlaneVirtualIP(client, cfg, v1alpha1.ClusterConfigVariableName, VariableName),
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
