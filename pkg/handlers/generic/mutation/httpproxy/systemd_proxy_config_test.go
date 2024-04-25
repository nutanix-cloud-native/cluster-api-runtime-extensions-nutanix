// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"testing"

	. "github.com/onsi/gomega"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestGenerateSystemdFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		vars             v1alpha1.HTTPProxy
		noProxy          []string
		expectedContents string
	}{
		{
			name: "no proxy configuration",
		}, {
			name: "all vars set",
			vars: v1alpha1.HTTPProxy{
				HTTP:  "http://example.com",
				HTTPS: "https://example.com",
				AdditionalNo: []string{
					"no-proxy.example.com",
				},
			},
			expectedContents: `[Service]
Environment="HTTP_PROXY=http://example.com"
Environment="http_proxy=http://example.com"
Environment="HTTPS_PROXY=https://example.com"
Environment="https_proxy=https://example.com"
Environment="NO_PROXY=no-proxy.example.com"
Environment="no_proxy=no-proxy.example.com"
`,
		}, {
			name: "http only",
			vars: v1alpha1.HTTPProxy{
				HTTP: "http://example.com",
			},
			expectedContents: `[Service]
Environment="HTTP_PROXY=http://example.com"
Environment="http_proxy=http://example.com"
`,
		}, {
			name: "https only",
			vars: v1alpha1.HTTPProxy{
				HTTPS: "https://example.com",
			},
			expectedContents: `[Service]
Environment="HTTPS_PROXY=https://example.com"
Environment="https_proxy=https://example.com"
`,
		}, {
			name: "no proxy only",
			vars: v1alpha1.HTTPProxy{
				AdditionalNo: []string{
					"no-proxy.example.com",
				},
			},
			expectedContents: `[Service]
Environment="NO_PROXY=no-proxy.example.com"
Environment="no_proxy=no-proxy.example.com"
`,
		}, {
			name: "multiple no proxy only",
			vars: v1alpha1.HTTPProxy{
				AdditionalNo: []string{
					"no-proxy.example.com",
					"no-proxy-1.example.com",
				},
			},
			expectedContents: `[Service]
Environment="NO_PROXY=no-proxy.example.com,no-proxy-1.example.com"
Environment="no_proxy=no-proxy.example.com,no-proxy-1.example.com"
`,
		}, {
			name: "default no proxy values",
			vars: v1alpha1.HTTPProxy{
				AdditionalNo: []string{
					"no-proxy.example.com",
					"no-proxy-1.example.com",
				},
			},
			noProxy: []string{"localhost", "127.0.0.1"},
			expectedContents: `[Service]
Environment="NO_PROXY=localhost,127.0.0.1,no-proxy.example.com,no-proxy-1.example.com"
Environment="no_proxy=localhost,127.0.0.1,no-proxy.example.com,no-proxy-1.example.com"
`,
		},
	}

	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			var expectedFiles []bootstrapv1.File
			if tt.expectedContents != "" {
				expectedFiles = []bootstrapv1.File{{
					Path:        systemdUnitPaths[0],
					Content:     tt.expectedContents,
					Permissions: "0640",
					Owner:       "root",
				}, {
					Path:        systemdUnitPaths[1],
					Content:     tt.expectedContents,
					Permissions: "0640",
					Owner:       "root",
				}}
			}

			g.Expect(generateSystemdFiles(tt.vars, append(tt.noProxy, tt.vars.AdditionalNo...))).
				Should(Equal(expectedFiles))
		})
	}
}
