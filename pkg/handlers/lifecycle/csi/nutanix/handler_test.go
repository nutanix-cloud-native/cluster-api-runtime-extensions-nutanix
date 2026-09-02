// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

const nutanixCSIValuesTemplate = `# Disable creating the Prism Central credentials Secret, the Secret will be created by the handler.
createPrismCentralSecret: false
# Disable creating the Prism Element credentials Secret, it won't be used the CSI driver as configured here.
createSecret: false
createPESecrets: false
pcSecretName: nutanix-csi-credentials
applyMpioConfigs: {{ .ApplyMpioConfigs }}
createVolumeSnapshotClass: {{ .CreateVolumeSnapshotClass }}
`

func Test_templateValuesFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   helmValuesInput
		want []string
	}{
		{
			name: "disables snapshot class when the v1 API is not served",
			in: helmValuesInput{
				ApplyMpioConfigs:          false,
				CreateVolumeSnapshotClass: false,
			},
			want: []string{
				"applyMpioConfigs: false",
				"createVolumeSnapshotClass: false",
			},
		},
		{
			name: "creates snapshot class when the v1 API is served",
			in: helmValuesInput{
				ApplyMpioConfigs:          true,
				CreateVolumeSnapshotClass: true,
			},
			want: []string{
				"applyMpioConfigs: true",
				"createVolumeSnapshotClass: true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := tt.in
			got, err := templateValuesFunc(&in)(&clusterv1.Cluster{}, nutanixCSIValuesTemplate)
			if err != nil {
				t.Fatalf("templateValuesFunc() error = %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("templateValuesFunc() missing %q in:\n%s", want, got)
				}
			}
		})
	}
}

func Test_snapshotControllerEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		csi  *apivariables.CSI
		want bool
	}{
		{
			name: "enabled",
			csi: &apivariables.CSI{
				GenericCSI: v1alpha1.GenericCSI{
					SnapshotController: &v1alpha1.SnapshotController{
						Strategy: v1alpha1.AddonStrategyHelmAddon,
					},
				},
				Providers: map[string]v1alpha1.CSIProvider{
					v1alpha1.CSIProviderNutanix: {},
				},
			},
			want: true,
		},
		{
			name: "omitted",
			csi: &apivariables.CSI{
				Providers: map[string]v1alpha1.CSIProvider{
					v1alpha1.CSIProviderNutanix: {},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cluster, err := clusterWithCSI(tt.csi)
			if err != nil {
				t.Fatalf("clusterWithCSI() error = %v", err)
			}
			got := snapshotControllerEnabled(cluster)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("snapshotControllerEnabled() mismatch (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("no topology", func(t *testing.T) {
		t.Parallel()
		if snapshotControllerEnabled(&clusterv1.Cluster{}) {
			t.Fatal("snapshotControllerEnabled() = true, want false")
		}
	})
}

func Test_volumeSnapshotV1Available_falseWithoutCRD(t *testing.T) {
	t.Parallel()

	if volumeSnapshotV1Available(fake.NewClientBuilder().Build()) {
		t.Fatal("volumeSnapshotV1Available() = true, want false when the CRD is not registered")
	}
}

func clusterWithCSI(csi *apivariables.CSI) (*clusterv1.Cluster, error) {
	cv, err := apivariables.MarshalToClusterVariable(
		v1alpha1.ClusterConfigVariableName,
		&apivariables.ClusterConfigSpec{
			Addons: &apivariables.Addons{
				CSI: csi,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	return &clusterv1.Cluster{
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: "dummy-class"},
				Variables: []clusterv1.ClusterVariable{
					*cv,
				},
			},
		},
	}, nil
}
