// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditlog

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	eksclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var testDefs = []capitest.VariableTestDef{
	{
		Name: "valid: unset AuditLog configuration",
		Vals: v1alpha1.GenericClusterConfigSpec{},
	},
	{
		Name: "valid: empty AuditLog configuration",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{},
		},
	},
	{
		Name: "valid: only webhook backend set",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Secret: &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
				},
			},
		},
	},
	{
		Name: "valid: only log backend set",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{},
			},
		},
	},
	{
		Name: "valid: only policy set",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Policy: &v1alpha1.AuditLogPolicy{
					ConfigMap: &v1alpha1.LocalObjectReference{Name: "audit-policy"},
				},
			},
		},
	},
	{
		Name: "valid: webhook backend with policy",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Secret: &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
				},
				Policy: &v1alpha1.AuditLogPolicy{
					ConfigMap: &v1alpha1.LocalObjectReference{Name: "audit-policy"},
				},
			},
		},
	},
	{
		Name: "valid: log backend with policy",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{},
				Policy: &v1alpha1.AuditLogPolicy{
					ConfigMap: &v1alpha1.LocalObjectReference{Name: "audit-policy"},
				},
			},
		},
	},
	{
		Name: "invalid: both webhook and log backends set",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Secret: &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
				},
				Log: &v1alpha1.AuditLogBackendLog{},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid: webhook backend missing required secret",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid: webhook backend secret name empty",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Secret: &v1alpha1.LocalObjectReference{Name: ""},
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid: policy missing required configMap",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Policy: &v1alpha1.AuditLogPolicy{},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid: policy configMap name empty",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Policy: &v1alpha1.AuditLogPolicy{
					ConfigMap: &v1alpha1.LocalObjectReference{Name: ""},
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "valid: webhook mode batch",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Mode:   "batch",
					Secret: &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
				},
			},
		},
	},
	{
		Name: "valid: webhook mode blocking",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Mode:   "blocking",
					Secret: &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
				},
			},
		},
	},
	{
		Name: "valid: webhook mode blocking-strict",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Mode:   "blocking-strict",
					Secret: &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
				},
			},
		},
	},
	{
		Name: "invalid: webhook mode not in enum",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Mode:   "invalid-mode",
					Secret: &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "valid: log mode batch",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{
					Mode: "batch",
				},
			},
		},
	},
	{
		Name: "valid: log mode blocking",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{
					Mode: "blocking",
				},
			},
		},
	},
	{
		Name: "valid: log mode blocking-strict",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{
					Mode: "blocking-strict",
				},
			},
		},
	},
	{
		Name: "invalid: log mode not in enum",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{
					Mode: "invalid-mode",
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "valid: webhook eventBatching set with mode batch",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Mode:          "batch",
					Secret:        &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
					EventBatching: &v1alpha1.AuditLogEventBatching{},
				},
			},
		},
	},
	{
		Name: "valid: webhook eventBatching set with defaulted mode (batch)",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Secret:        &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
					EventBatching: &v1alpha1.AuditLogEventBatching{},
				},
			},
		},
	},
	{
		Name: "invalid: webhook eventBatching set with mode blocking",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Mode:          "blocking",
					Secret:        &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
					EventBatching: &v1alpha1.AuditLogEventBatching{},
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid: webhook eventBatching set with mode blocking-strict",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Mode:          "blocking-strict",
					Secret:        &v1alpha1.LocalObjectReference{Name: "audit-webhook"},
					EventBatching: &v1alpha1.AuditLogEventBatching{},
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "valid: log eventBatching set with mode batch",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{
					Mode:          "batch",
					EventBatching: &v1alpha1.AuditLogEventBatching{},
				},
			},
		},
	},
	{
		Name: "invalid: log eventBatching set with defaulted mode (blocking)",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{
					EventBatching: &v1alpha1.AuditLogEventBatching{},
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid: log eventBatching set with mode blocking",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{
					Mode:          "blocking",
					EventBatching: &v1alpha1.AuditLogEventBatching{},
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid: log eventBatching set with mode blocking-strict",
		Vals: v1alpha1.GenericClusterConfigSpec{
			AuditLog: &v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{
					Mode:          "blocking-strict",
					EventBatching: &v1alpha1.AuditLogEventBatching{},
				},
			},
		},
		ExpectError: true,
	},
}

func TestVariableValidation_AuditLog_AWS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_AuditLog_Docker(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_AuditLog_Nutanix(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_AuditLog_EKS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.EKSClusterConfig{}.VariableSchema()),
		true,
		eksclusterconfig.NewVariable,
		testDefs...,
	)
}
