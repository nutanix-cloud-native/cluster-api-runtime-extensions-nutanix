// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cncfdistribution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
)

const (
	testNamespace       = "registry-system"
	testStatefulSetName = "cncf-distribution-registry-docker-registry"
	testHelmReleaseName = "cncf-distribution-registry"
)

func Test_expectedPersistenceSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		hcp     *caaphv1.HelmChartProxy
		want    string
		wantErr string
	}{
		{
			name: "empty values template returns empty string",
			hcp:  testHelmChartProxy(""),
			want: "",
		},
		{
			name: "persistence not enabled returns empty string",
			hcp: testHelmChartProxy(`
persistence:
  enabled: false
  size: "50Gi"
`),
			want: "",
		},
		{
			name: "persistence enabled returns size",
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "50Gi"
`),
			want: "50Gi",
		},
		{
			name: "persistence enabled with different size",
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "100Gi"
`),
			want: "100Gi",
		},
		{
			name: "no persistence key returns empty string",
			hcp: testHelmChartProxy(`
replicaCount: 2
`),
			want: "",
		},
		{
			name: "persistence enabled but no size returns empty string",
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
`),
			want: "",
		},
		{
			name:    "invalid YAML returns error",
			hcp:     testHelmChartProxy(`invalid: yaml: content`),
			wantErr: "failed to parse values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := expectedPersistenceSize(tt.hcp)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_updateStatefulSetVolumeClaimTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		cluster                *clusterv1.Cluster
		hcp                    *caaphv1.HelmChartProxy
		existingStatefulSet    *appsv1.StatefulSet
		wantStatefulSetDeleted bool
		wantErr                string
	}{
		{
			name:                   "empty values template does nothing",
			cluster:                testCluster(),
			hcp:                    testHelmChartProxy(""),
			existingStatefulSet:    testStatefulSet("10Gi"),
			wantStatefulSetDeleted: false,
		},
		{
			name:    "persistence not enabled does nothing",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: false
  size: "50Gi"
`),
			existingStatefulSet:    testStatefulSet("10Gi"),
			wantStatefulSetDeleted: false,
		},
		{
			name:    "same storage size does nothing",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "10Gi"
`),
			existingStatefulSet:    testStatefulSet("10Gi"),
			wantStatefulSetDeleted: false,
		},
		{
			name:    "different storage size deletes StatefulSet",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "50Gi"
`),
			existingStatefulSet:    testStatefulSet("10Gi"),
			wantStatefulSetDeleted: true,
		},
		{
			name:    "larger storage size deletes StatefulSet",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "100Gi"
`),
			existingStatefulSet:    testStatefulSet("50Gi"),
			wantStatefulSetDeleted: true,
		},
		{
			name:    "StatefulSet without volumeClaimTemplates does nothing",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "50Gi"
`),
			existingStatefulSet: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testStatefulSetName,
					Namespace: testNamespace,
				},
				Spec: appsv1.StatefulSetSpec{
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{},
				},
			},
			wantStatefulSetDeleted: false,
		},
		{
			name:    "StatefulSet not found does nothing",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "50Gi"
`),
			existingStatefulSet:    nil,
			wantStatefulSetDeleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var objs []client.Object
			if tt.existingStatefulSet != nil {
				objs = append(objs, tt.existingStatefulSet)
			}
			remoteClient := buildFakeClient(t, objs...)

			ctx := context.Background()
			err := updateStatefulSetVolumeClaimTemplate(ctx, nil, remoteClient, tt.cluster, tt.hcp)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)

			// Verify StatefulSet state
			sts := &appsv1.StatefulSet{}
			getErr := remoteClient.Get(ctx, client.ObjectKey{
				Name:      testStatefulSetName,
				Namespace: testNamespace,
			}, sts)

			if tt.wantStatefulSetDeleted {
				assert.True(t, client.IgnoreNotFound(getErr) == nil && getErr != nil,
					"expected StatefulSet to be deleted")
			} else if tt.existingStatefulSet != nil {
				require.NoError(t, getErr, "expected StatefulSet to still exist")
			}
		})
	}
}

func Test_expandPersistentVolumeClaims(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cluster      *clusterv1.Cluster
		hcp          *caaphv1.HelmChartProxy
		existingPVCs []*corev1.PersistentVolumeClaim
		wantSizes    map[string]string // PVC name: expected storage size
		wantErr      string
	}{
		{
			name:    "empty values template does nothing",
			cluster: testCluster(),
			hcp:     testHelmChartProxy(""),
			existingPVCs: []*corev1.PersistentVolumeClaim{
				testPVC("data-sts-0", "10Gi"),
			},
			wantSizes: map[string]string{
				"data-sts-0": "10Gi",
			},
		},
		{
			name:    "persistence not enabled does nothing",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: false
  size: "50Gi"
`),
			existingPVCs: []*corev1.PersistentVolumeClaim{
				testPVC("data-sts-0", "10Gi"),
			},
			wantSizes: map[string]string{
				"data-sts-0": "10Gi",
			},
		},
		{
			name:    "no PVCs found does nothing",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "50Gi"
`),
			existingPVCs: nil,
			wantSizes:    nil,
		},
		{
			name:    "same storage size does nothing",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "10Gi"
`),
			existingPVCs: []*corev1.PersistentVolumeClaim{
				testPVC("data-sts-0", "10Gi"),
			},
			wantSizes: map[string]string{
				"data-sts-0": "10Gi",
			},
		},
		{
			name:    "expands single PVC to larger size",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "50Gi"
`),
			existingPVCs: []*corev1.PersistentVolumeClaim{
				testPVC("data-sts-0", "10Gi"),
			},
			wantSizes: map[string]string{
				"data-sts-0": "50Gi",
			},
		},
		{
			name:    "expands multiple PVCs to larger size",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "100Gi"
`),
			existingPVCs: []*corev1.PersistentVolumeClaim{
				testPVC("data-sts-0", "10Gi"),
				testPVC("data-sts-1", "10Gi"),
			},
			wantSizes: map[string]string{
				"data-sts-0": "100Gi",
				"data-sts-1": "100Gi",
			},
		},
		{
			name:    "only expands PVCs that need resizing",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "50Gi"
`),
			existingPVCs: []*corev1.PersistentVolumeClaim{
				testPVC("data-sts-0", "10Gi"),
				testPVC("data-sts-1", "50Gi"),
			},
			wantSizes: map[string]string{
				"data-sts-0": "50Gi",
				"data-sts-1": "50Gi",
			},
		},
		{
			name:    "PVC without storage request is skipped",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "50Gi"
`),
			existingPVCs: []*corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "data-sts-0",
						Namespace: testNamespace,
						Labels: map[string]string{
							"release": testHelmReleaseName,
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{},
						},
					},
				},
			},
			wantSizes: map[string]string{},
		},
		{
			name:    "invalid persistence size returns error",
			cluster: testCluster(),
			hcp: testHelmChartProxy(`
persistence:
  enabled: true
  size: "invalid"
`),
			existingPVCs: []*corev1.PersistentVolumeClaim{
				testPVC("data-sts-0", "10Gi"),
			},
			wantErr: "failed to parse expected persistence size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objs := make([]client.Object, 0, len(tt.existingPVCs))
			for _, pvc := range tt.existingPVCs {
				objs = append(objs, pvc)
			}
			remoteClient := buildFakeClient(t, objs...)

			ctx := context.Background()
			err := expandPersistentVolumeClaims(ctx, nil, remoteClient, tt.cluster, tt.hcp)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)

			// Verify PVC sizes
			for pvcName, expectedSize := range tt.wantSizes {
				pvc := &corev1.PersistentVolumeClaim{}
				err := remoteClient.Get(ctx, client.ObjectKey{
					Name:      pvcName,
					Namespace: testNamespace,
				}, pvc)
				require.NoError(t, err, "failed to get PVC %s", pvcName)

				storage := pvc.Spec.Resources.Requests.Storage()
				if storage != nil {
					assert.Equal(t, expectedSize, storage.String(),
						"PVC %s has unexpected storage size", pvcName)
				}
			}
		})
	}
}

func buildFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func testCluster() *clusterv1.Cluster {
	return &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: corev1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{"10.96.0.0/12"},
				},
			},
		},
	}
}

func testStatefulSet(storageSize string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testStatefulSetName,
			Namespace: testNamespace,
		},
		Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(storageSize),
							},
						},
					},
				},
			},
		},
	}
}

func testPVC(name, storageSize string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels: map[string]string{
				"release": testHelmReleaseName,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		},
	}
}

func testHelmChartProxy(valuesTemplate string) *caaphv1.HelmChartProxy {
	return &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: corev1.NamespaceDefault,
		},
		Spec: caaphv1.HelmChartProxySpec{
			ValuesTemplate: valuesTemplate,
		},
	}
}
