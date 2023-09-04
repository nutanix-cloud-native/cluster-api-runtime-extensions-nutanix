// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"testing"

	. "github.com/onsi/gomega"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

func TestGenerateSystemdFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		vars             HTTPProxyVariables
		expectedContents string
	}{{
		name: "no proxy configuration",
	}, {
		name: "all vars set",
		vars: HTTPProxyVariables{
			HTTP:  "http://example.com",
			HTTPS: "https://example.com",
			No: []string{
				"https://no-proxy.example.com",
			},
		},
		expectedContents: `[Service]
Environment="HTTP_PROXY=http://example.com"
Environment="http_proxy=http://example.com"
Environment="HTTPS_PROXY=https://example.com"
Environment="https_proxy=https://example.com"
Environment="NO_PROXY=https://no-proxy.example.com"
Environment="no_proxy=https://no-proxy.example.com"
`,
	}, {
		name: "http only",
		vars: HTTPProxyVariables{
			HTTP: "http://example.com",
		},
		expectedContents: `[Service]
Environment="HTTP_PROXY=http://example.com"
Environment="http_proxy=http://example.com"
`,
	}, {
		name: "https only",
		vars: HTTPProxyVariables{
			HTTPS: "https://example.com",
		},
		expectedContents: `[Service]
Environment="HTTPS_PROXY=https://example.com"
Environment="https_proxy=https://example.com"
`,
	}, {
		name: "no proxy only",
		vars: HTTPProxyVariables{
			No: []string{
				"https://no-proxy.example.com",
			},
		},
		expectedContents: `[Service]
Environment="NO_PROXY=https://no-proxy.example.com"
Environment="no_proxy=https://no-proxy.example.com"
`,
	}, {
		name: "multiple no proxy only",
		vars: HTTPProxyVariables{
			No: []string{
				"https://no-proxy.example.com",
				"https://no-proxy-1.example.com",
			},
		},
		expectedContents: `[Service]
Environment="NO_PROXY=https://no-proxy.example.com,https://no-proxy-1.example.com"
Environment="no_proxy=https://no-proxy.example.com,https://no-proxy-1.example.com"
`,
	}}

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

			g.Expect(generateSystemdFiles(tt.vars)).Should(Equal(expectedFiles))
		})
	}
}
