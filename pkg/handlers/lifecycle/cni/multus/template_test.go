// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func Test_templateValuesFunc(t *testing.T) {
	tests := []struct {
		name                   string
		cniVar                 v1alpha1.CNI
		valuesTemplate         string
		expectedRenderedOutput string
		wantErr                bool
	}{
		{
			name: "Cilium socket path templates correctly",
			cniVar: v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
			},
			valuesTemplate: `daemonConfig:
  readinessIndicatorFile: "{{ .ReadinessSocketPath }}"
{{- if .ReadinessSocketPath }}
volumes:
  - name: cni-readiness-sock
    hostPath:
      path: "{{ .ReadinessSocketPath }}"
      type: Socket
volumeMounts:
  - name: cni-readiness-sock
    mountPath: "{{ .ReadinessSocketPath }}"
    readOnly: true
{{- end }}`,
			expectedRenderedOutput: `daemonConfig:
  readinessIndicatorFile: "/run/cilium/cilium.sock"
volumes:
  - name: cni-readiness-sock
    hostPath:
      path: "/run/cilium/cilium.sock"
      type: Socket
volumeMounts:
  - name: cni-readiness-sock
    mountPath: "/run/cilium/cilium.sock"
    readOnly: true`,
			wantErr: false,
		},
		{
			name: "Calico socket path templates correctly",
			cniVar: v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCalico,
			},
			valuesTemplate: `daemonConfig:
  readinessIndicatorFile: "{{ .ReadinessSocketPath }}"
{{- if .ReadinessSocketPath }}
volumes:
  - name: cni-readiness-sock
    hostPath:
      path: "{{ .ReadinessSocketPath }}"
      type: Socket
volumeMounts:
  - name: cni-readiness-sock
    mountPath: "{{ .ReadinessSocketPath }}"
    readOnly: true
{{- end }}`,
			expectedRenderedOutput: `daemonConfig:
  readinessIndicatorFile: "/var/run/calico/cni-server.sock"
volumes:
  - name: cni-readiness-sock
    hostPath:
      path: "/var/run/calico/cni-server.sock"
      type: Socket
volumeMounts:
  - name: cni-readiness-sock
    mountPath: "/var/run/calico/cni-server.sock"
    readOnly: true`,
			wantErr: false,
		},
		{
			name: "Unsupported CNI provider returns error",
			cniVar: v1alpha1.CNI{
				Provider: "UnsupportedCNI",
			},
			valuesTemplate: `daemonConfig:
  readinessIndicatorFile: "{{ .ReadinessSocketPath }}"`,
			expectedRenderedOutput: "",
			wantErr:                true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateFunc := templateValuesFunc(tt.cniVar)

			var cluster clusterv1.Cluster
			got, err := templateFunc(&cluster, tt.valuesTemplate)

			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedRenderedOutput, got)
			}
		})
	}
}
