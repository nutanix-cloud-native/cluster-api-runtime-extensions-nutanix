// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditlog

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateAuditPolicyDocument(t *testing.T) {
	t.Parallel()

	validYAML := `apiVersion: audit.k8s.io/v1
kind: Policy
rules: []
`
	validJSON := `{"apiVersion":"audit.k8s.io/v1","kind":"Policy","rules":[]}`

	tests := []struct {
		name    string
		doc     string
		wantErr string
	}{
		{
			name: "valid yaml",
			doc:  validYAML,
		},
		{
			name: "valid json",
			doc:  validJSON,
		},
		{
			name:    "empty",
			doc:     "",
			wantErr: "empty",
		},
		{
			name:    "whitespace only",
			doc:     " \n\t ",
			wantErr: "empty",
		},
		{
			name: "wrong apiVersion",
			doc: `apiVersion: audit.k8s.io/v1alpha1
kind: Policy
rules: []
`,
			wantErr: "apiVersion",
		},
		{
			name: "wrong kind",
			doc: `apiVersion: audit.k8s.io/v1
kind: PolicyList
rules: []
`,
			wantErr: "kind",
		},
		{
			name: "unknown field strict",
			doc: `apiVersion: audit.k8s.io/v1
kind: Policy
rules: []
notARealField: true
`,
			wantErr: "unknown field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAuditPolicyDocument(tt.doc)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateAuditPolicyDocument: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantErr)) {
				t.Fatalf("error %q should mention %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPolicyFromConfigMap(t *testing.T) {
	t.Parallel()

	valid := `apiVersion: audit.k8s.io/v1
kind: Policy
rules: []
`

	tests := []struct {
		name             string
		cm               *corev1.ConfigMap
		wantErrSubstring string
	}{
		{
			name: "ok",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "p"},
				Data: map[string]string{
					AuditPolicyDataKey: valid,
				},
			},
		},
		{
			name: "missing key",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "p"},
				Data: map[string]string{
					"wrong.yaml": valid,
				},
			},
			wantErrSubstring: "no data",
		},
		{
			name: "empty policy string",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "p"},
				Data: map[string]string{
					AuditPolicyDataKey: " \n",
				},
			},
			wantErrSubstring: "empty data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := policyFromConfigMap(tt.cm)
			if tt.wantErrSubstring == "" {
				if err != nil {
					t.Fatalf("policyFromConfigMap: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstring) {
				t.Fatalf("error %q should contain %q", err.Error(), tt.wantErrSubstring)
			}
		})
	}
}
