// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanevirtualip

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestControlPlaneEndpointPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane virtual IP suite")
}

var _ = Describe("Generate ControlPlane virtual IP patches", func() {
	requestItemBuilder := request.KubeadmControlPlaneTemplateRequestItemBuilder{}
	requestItem := requestItemBuilder.WithFiles(bootstrapv1.File{
		Path:    "/etc/kubernetes/manifests/kube-vip.yaml",
		Content: validKubeVIPTemplate,
	}).NewRequest("")

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
				RequestItem: requestItem,
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
					{
						Operation:    "add",
						Path:         "/spec/template/spec/kubeadmConfigSpec/files/0/owner",
						ValueMatcher: gomega.Equal("root:root"),
					},
					{
						Operation:    "add",
						Path:         "/spec/template/spec/kubeadmConfigSpec/files/0/permissions",
						ValueMatcher: gomega.Equal("0600"),
					},
					{
						Operation: "replace",
						Path:      "/spec/template/spec/kubeadmConfigSpec/files/0/content",
						ValueMatcher: gomega.SatisfyAll(
							gomega.ContainSubstring("value: \"10.20.100.10\""),
							gomega.ContainSubstring("value: \"6443\""),
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
					Topology: clusterv1.Topology{
						ClassRef: clusterv1.ClusterClassRef{Name: "dummy-class"},
						Version:  "v1.28.100",
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
				RequestItem: requestItem,
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
					{
						Operation:    "add",
						Path:         "/spec/template/spec/kubeadmConfigSpec/files/0/owner",
						ValueMatcher: gomega.Equal("root:root"),
					},
					{
						Operation:    "add",
						Path:         "/spec/template/spec/kubeadmConfigSpec/files/0/permissions",
						ValueMatcher: gomega.Equal("0600"),
					},
					{
						Operation: "replace",
						Path:      "/spec/template/spec/kubeadmConfigSpec/files/0/content",
						ValueMatcher: gomega.SatisfyAll(
							gomega.ContainSubstring("value: \"10.20.100.10\""),
							gomega.ContainSubstring("value: \"6443\""),
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
					Topology: clusterv1.Topology{
						ClassRef: clusterv1.ClusterClassRef{Name: "dummy-class"},
						Version:  "v1.29.0",
					},
				},
			},
		},
		{
			PatchTestDef: capitest.PatchTestDef{
				Name: "template file should be removed when virtual IP is not set",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.ControlPlaneEndpointSpec{
							Host: "10.20.100.10",
							Port: 6443,
						},
						VariableName,
					),
				},
				RequestItem: requestItem,
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
					{
						Operation:    "remove",
						Path:         "/spec/template/spec/kubeadmConfigSpec/files",
						ValueMatcher: gomega.BeNil(),
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
					Topology: clusterv1.Topology{
						ClassRef: clusterv1.ClusterClassRef{Name: "dummy-class"},
						Version:  "v1.28.100",
					},
				},
			},
		},
	}

	// create test node for each case
	for _, tt := range testDefs {
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

			patchGenerator := func() mutation.GeneratePatches {
				return mutation.NewMetaGeneratePatchesHandler(
					"",
					client,
					NewControlPlaneVirtualIP(v1alpha1.ClusterConfigVariableName, VariableName),
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
          value: "{{ .Address }}"
        - name: port
          value: "{{ .Port }}"
`

func Test_deleteFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		obj           *controlplanev1.KubeadmControlPlaneTemplate
		filesToDelete []string
		expectedFiles []bootstrapv1.File
	}{
		{
			name: "should delete files from the template",
			obj: &controlplanev1.KubeadmControlPlaneTemplate{
				Spec: controlplanev1.KubeadmControlPlaneTemplateSpec{
					Template: controlplanev1.KubeadmControlPlaneTemplateResource{
						Spec: controlplanev1.KubeadmControlPlaneTemplateResourceSpec{
							KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
								Files: []bootstrapv1.File{
									{
										Path: "file-1",
									},
									{
										Path: "file-2",
									},
									{
										Path: "file-3",
									},
									{
										Path: "file-4",
									},
								},
							},
						},
					},
				},
			},
			filesToDelete: []string{"file-2", "file-3", "file-5"},
			expectedFiles: []bootstrapv1.File{
				{
					Path: "file-1",
				},
				{
					Path: "file-4",
				},
			},
		},
		{
			name: "should keep all files",
			obj: &controlplanev1.KubeadmControlPlaneTemplate{
				Spec: controlplanev1.KubeadmControlPlaneTemplateSpec{
					Template: controlplanev1.KubeadmControlPlaneTemplateResource{
						Spec: controlplanev1.KubeadmControlPlaneTemplateResourceSpec{
							KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
								Files: []bootstrapv1.File{
									{
										Path: "file-1",
									},
									{
										Path: "file-2",
									},
									{
										Path: "file-3",
									},
									{
										Path: "file-4",
									},
								},
							},
						},
					},
				},
			},
			filesToDelete: []string{"file-5"},
			expectedFiles: []bootstrapv1.File{
				{
					Path: "file-1",
				},
				{
					Path: "file-2",
				},
				{
					Path: "file-3",
				},
				{
					Path: "file-4",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			updatedFiles := deleteFiles(
				tt.obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				tt.filesToDelete...,
			)
			assert.Equal(t, tt.expectedFiles, updatedFiles)
		})
	}
}

func Test_mergeFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		obj           *controlplanev1.KubeadmControlPlaneTemplate
		files         []bootstrapv1.File
		expectedFiles []bootstrapv1.File
	}{
		{
			name: "should merge files",
			obj: &controlplanev1.KubeadmControlPlaneTemplate{
				Spec: controlplanev1.KubeadmControlPlaneTemplateSpec{
					Template: controlplanev1.KubeadmControlPlaneTemplateResource{
						Spec: controlplanev1.KubeadmControlPlaneTemplateResourceSpec{
							KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
								Files: []bootstrapv1.File{
									{
										Path:    "file-1",
										Content: "old",
									},
									{
										Path:    "file-2",
										Content: "old",
									},
									{
										Path:    "file-3",
										Content: "old",
									},
									{
										Path:    "file-4",
										Content: "old",
									},
								},
							},
						},
					},
				},
			},
			files: []bootstrapv1.File{
				{
					Path:    "file-1",
					Content: "new",
				},
				{
					Path:    "file-4",
					Content: "new",
				},
				{
					Path:    "file-5",
					Content: "new",
				},
			},
			expectedFiles: []bootstrapv1.File{
				{
					Path:    "file-1",
					Content: "new",
				},
				{
					Path:    "file-2",
					Content: "old",
				},
				{
					Path:    "file-3",
					Content: "old",
				},
				{
					Path:    "file-4",
					Content: "new",
				},
				{
					Path:    "file-5",
					Content: "new",
				},
			},
		},
		{
			name: "should add a new file",
			obj: &controlplanev1.KubeadmControlPlaneTemplate{
				Spec: controlplanev1.KubeadmControlPlaneTemplateSpec{
					Template: controlplanev1.KubeadmControlPlaneTemplateResource{
						Spec: controlplanev1.KubeadmControlPlaneTemplateResourceSpec{
							KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
								Files: []bootstrapv1.File{
									{
										Path:    "file-1",
										Content: "old",
									},
									{
										Path:    "file-2",
										Content: "old",
									},
									{
										Path:    "file-3",
										Content: "old",
									},
									{
										Path:    "file-4",
										Content: "old",
									},
								},
							},
						},
					},
				},
			},
			files: []bootstrapv1.File{
				{
					Path:    "file-5",
					Content: "new",
				},
			},
			expectedFiles: []bootstrapv1.File{
				{
					Path:    "file-1",
					Content: "old",
				},
				{
					Path:    "file-2",
					Content: "old",
				},
				{
					Path:    "file-3",
					Content: "old",
				},
				{
					Path:    "file-4",
					Content: "old",
				},
				{
					Path:    "file-5",
					Content: "new",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			updatedFiles := mergeFiles(
				tt.obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				tt.files...,
			)
			assert.Equal(t, tt.expectedFiles, updatedFiles)
		})
	}
}
