// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func Test_GenerateFilesAndCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                        string
		controlPlaneEndpointSpec    v1alpha1.ControlPlaneEndpointSpec
		cluster                     *clusterv1.Cluster
		kcp                         *controlplanev1.KubeadmControlPlaneTemplate
		expectedFiles               []bootstrapv1.File
		expectedPreKubeadmCommands  []string
		expectedPostKubeadmCommands []string
		expectedErr                 error
	}{
		{
			name: "should return templated data with both host and port and pre/post kubeadm hack commands",
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "10.20.100.10",
				Port: 6443,
			},
			kcp: kubeadmControlPlaneTemplateWithFiles(
				t, []bootstrapv1.File{{
					Path:    kubeVIPFilePath,
					Content: validKubeVIPTemplate,
				}}),
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "v1.29.0",
					},
				},
			},
			expectedFiles: []bootstrapv1.File{
				{
					Content:     expectedKubeVIPPod,
					Owner:       kubeVIPFileOwner,
					Path:        kubeVIPFilePath,
					Permissions: kubeVIPFilePermissions,
				},
				{
					Content:     string(configureForKubeVIPScript),
					Path:        configureForKubeVIPScriptOnRemote,
					Permissions: configureForKubeVIPScriptPermissions,
				},
			},
			expectedPreKubeadmCommands: []string{
				configureForKubeVIPScriptOnRemotePreKubeadmCommand,
			},
			expectedPostKubeadmCommands: []string{
				configureForKubeVIPScriptOnRemotePostKubeadmCommand,
			},
		},
		{
			name: "should return templated data with both host and port",
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "10.20.100.10",
				Port: 6443,
			},
			kcp: kubeadmControlPlaneTemplateWithFiles(
				t, []bootstrapv1.File{{
					Path:    kubeVIPFilePath,
					Content: validKubeVIPTemplate,
				}}),
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "v1.28.0",
					},
				},
			},
			expectedFiles: []bootstrapv1.File{
				{
					Content:     expectedKubeVIPPod,
					Owner:       kubeVIPFileOwner,
					Path:        kubeVIPFilePath,
					Permissions: kubeVIPFilePermissions,
				},
			},
		},
		{
			name: "should return templated data with both IP and port from virtual IP configuration overrides",
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "10.20.100.10",
				Port: 6443,
				VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{
					Configuration: &v1alpha1.ControlPlaneVirtualIPConfiguration{
						Address: "172.20.100.10",
						Port:    8443,
					},
				},
			},
			kcp: kubeadmControlPlaneTemplateWithFiles(
				t, []bootstrapv1.File{{
					Path:    kubeVIPFilePath,
					Content: validKubeVIPTemplate,
				}}),
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "v1.28.0",
					},
				},
			},
			expectedFiles: []bootstrapv1.File{
				{
					Content:     expectedKubeVIPPodWithOverrides,
					Owner:       kubeVIPFileOwner,
					Path:        kubeVIPFilePath,
					Permissions: kubeVIPFilePermissions,
				},
			},
		},
		{
			name: "should return templated data with IP from virtual IP configuration overrides",
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "10.20.100.10",
				Port: 8443,
				VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{
					Configuration: &v1alpha1.ControlPlaneVirtualIPConfiguration{
						Address: "172.20.100.10",
					},
				},
			},
			kcp: kubeadmControlPlaneTemplateWithFiles(
				t, []bootstrapv1.File{{
					Path:    kubeVIPFilePath,
					Content: validKubeVIPTemplate,
				}}),
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "dummy-class",
						Version: "v1.28.0",
					},
				},
			},
			expectedFiles: []bootstrapv1.File{
				{
					Content:     expectedKubeVIPPodWithOverrides,
					Owner:       kubeVIPFileOwner,
					Path:        kubeVIPFilePath,
					Permissions: kubeVIPFilePermissions,
				},
			},
		},
	}

	for idx := range tests {
		tt := tests[idx] // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := kubeVIPFromKCPTemplateProvider{
				template: tt.kcp,
			}

			files, preKubeadmCommands, postKubeadmCommands, err := provider.GenerateFilesAndCommands(
				context.Background(),
				tt.controlPlaneEndpointSpec,
				tt.cluster,
			)
			require.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedFiles, files)
			assert.Equal(t, tt.expectedPreKubeadmCommands, preKubeadmCommands)
			assert.Equal(t, tt.expectedPostKubeadmCommands, postKubeadmCommands)
		})
	}
}

func Test_getTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		kcp          *controlplanev1.KubeadmControlPlaneTemplate
		expectedData string
		expectedErr  error
	}{
		{
			name: "should return data from the only file",
			kcp: kubeadmControlPlaneTemplateWithFiles(
				t, []bootstrapv1.File{{
					Path:    kubeVIPFilePath,
					Content: "kube-vip-template",
				}}),
			expectedData: "kube-vip-template",
		},
		{
			name: "should fail with missingTemplateError",
			kcp: kubeadmControlPlaneTemplateWithFiles(
				t, []bootstrapv1.File{{
					Path:    "some-other-file",
					Content: "content",
				}}),
			expectedErr: missingTemplateError{
				path: kubeVIPFilePath,
			},
		},
	}

	for idx := range tests {
		tt := tests[idx] // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := getTemplate(tt.kcp)
			require.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedData, data)
		})
	}
}

func kubeadmControlPlaneTemplateWithFiles(
	t *testing.T, files []bootstrapv1.File,
) *controlplanev1.KubeadmControlPlaneTemplate {
	t.Helper()
	return &controlplanev1.KubeadmControlPlaneTemplate{
		Spec: controlplanev1.KubeadmControlPlaneTemplateSpec{
			Template: controlplanev1.KubeadmControlPlaneTemplateResource{
				Spec: controlplanev1.KubeadmControlPlaneTemplateResourceSpec{
					KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
						Files: files,
					},
				},
			},
		},
	}
}

var (
	validKubeVIPTemplate = `
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

	expectedKubeVIPPod = `
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
          value: "10.20.100.10"
        - name: port
          value: "6443"
`

	expectedKubeVIPPodWithOverrides = `
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
          value: "172.20.100.10"
        - name: port
          value: "8443"
`
)
