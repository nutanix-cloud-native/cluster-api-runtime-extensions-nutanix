// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditlog

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	sigyaml "sigs.k8s.io/yaml"
)

// policyFromConfigMap extracts the audit policy from the ConfigMap, validates it, and returns it.
func policyFromConfigMap(cm *corev1.ConfigMap) (string, error) {
	raw, ok := cm.Data[AuditPolicyDataKey]
	if !ok {
		return "", fmt.Errorf(
			"audit policy ConfigMap %q has no data for key %q",
			cm.Name,
			AuditPolicyDataKey,
		)
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("audit policy ConfigMap %q has empty data for key %q", cm.Name, AuditPolicyDataKey)
	}

	if err := validateAuditPolicyDocument(trimmed); err != nil {
		return "", fmt.Errorf("audit policy in ConfigMap %q is not valid: %w", cm.Name, err)
	}

	return trimmed, nil
}

// validateAuditPolicyDocument checks that data is YAML or JSON describing an audit.k8s.io/v1 Policy.
// It uses strict decoding so unknown fields are rejected.
func validateAuditPolicyDocument(data string) error {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return fmt.Errorf("audit policy document is empty")
	}

	var policy auditv1.Policy
	if err := sigyaml.UnmarshalStrict([]byte(trimmed), &policy); err != nil {
		return fmt.Errorf("document is not a valid audit.k8s.io/v1 Policy: %w", err)
	}

	if policy.APIVersion != auditv1.SchemeGroupVersion.String() {
		return fmt.Errorf(
			"invalid audit policy apiVersion %q, must be %q",
			policy.APIVersion,
			auditv1.SchemeGroupVersion.String(),
		)
	}
	if policy.Kind != "Policy" {
		return fmt.Errorf("invalid audit policy kind %q, must be Policy", policy.Kind)
	}

	return nil
}
