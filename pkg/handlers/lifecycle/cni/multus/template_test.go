// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Test_templateValuesFunc(t *testing.T) {
	tests := []struct {
		name                   string
		readinessSocketPath    string
		valuesTemplate         string
		expectedRenderedOutput string
		wantErr                bool
	}{
		{
			name:                "Cilium socket path templates correctly",
			readinessSocketPath: "/run/cilium/cilium.sock",
			valuesTemplate: `daemonConfig:
  readinessIndicatorFile: "{{ .ReadinessSocketPath }}"
{{- if .ReadinessSocketPath }}
volumes:
  - name: {{ .SocketVolumeName }}
    hostPath:
      path: "{{ .ReadinessSocketPath }}"
      type: Socket
volumeMounts:
  - name: {{ .SocketVolumeName }}
    mountPath: "{{ .ReadinessSocketPath }}"
    readOnly: true
{{- end }}`,
			expectedRenderedOutput: `daemonConfig:
  readinessIndicatorFile: "/run/cilium/cilium.sock"
volumes:
  - name: cilium-sock
    hostPath:
      path: "/run/cilium/cilium.sock"
      type: Socket
volumeMounts:
  - name: cilium-sock
    mountPath: "/run/cilium/cilium.sock"
    readOnly: true`,
			wantErr: false,
		},
		{
			name:                "Calico socket path templates correctly",
			readinessSocketPath: "/var/run/calico/cni-server.sock",
			valuesTemplate: `daemonConfig:
  readinessIndicatorFile: "{{ .ReadinessSocketPath }}"
{{- if .ReadinessSocketPath }}
volumes:
  - name: {{ .SocketVolumeName }}
    hostPath:
      path: "{{ .ReadinessSocketPath }}"
      type: Socket
volumeMounts:
  - name: {{ .SocketVolumeName }}
    mountPath: "{{ .ReadinessSocketPath }}"
    readOnly: true
{{- end }}`,
			expectedRenderedOutput: `daemonConfig:
  readinessIndicatorFile: "/var/run/calico/cni-server.sock"
volumes:
  - name: calico-sock
    hostPath:
      path: "/var/run/calico/cni-server.sock"
      type: Socket
volumeMounts:
  - name: calico-sock
    mountPath: "/var/run/calico/cni-server.sock"
    readOnly: true`,
			wantErr: false,
		},
		{
			name:                "Empty socket path handles gracefully",
			readinessSocketPath: "",
			valuesTemplate: `daemonConfig:
  readinessIndicatorFile: "{{ .ReadinessSocketPath }}"
{{- if .ReadinessSocketPath }}
volumes:
  - name: {{ .SocketVolumeName }}
    hostPath:
      path: "{{ .ReadinessSocketPath }}"
      type: Socket
{{- end }}`,
			expectedRenderedOutput: `daemonConfig:
  readinessIndicatorFile: ""`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateFunc := templateValuesFunc(tt.readinessSocketPath)

			var cluster clusterv1.Cluster
			got, err := templateFunc(&cluster, tt.valuesTemplate)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedRenderedOutput, got)
			}
		})
	}
}

func Test_extractVolumeName(t *testing.T) {
	tests := []struct {
		name         string
		socketPath   string
		expectedName string
	}{
		{
			name:         "Cilium socket path extracts correct volume name",
			socketPath:   "/run/cilium/cilium.sock",
			expectedName: "cilium-sock",
		},
		{
			name:         "Calico socket path extracts correct volume name",
			socketPath:   "/var/run/calico/cni-server.sock",
			expectedName: "calico-sock",
		},
		{
			name:         "Empty socket path returns empty string",
			socketPath:   "",
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVolumeName(tt.socketPath)
			assert.Equal(t, tt.expectedName, got)
		})
	}
}
