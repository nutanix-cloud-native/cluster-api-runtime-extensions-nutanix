// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// AuditLog defines the audit log configuration for the cluster.
// +kubebuilder:validation:XValidation:rule="!(has(self.webhook) && has(self.log))",message="Only one of 'webhook' or 'log' can be set"
type AuditLog struct {
	// Webhook defines the audit log webhook configuration.
	// +kubebuilder:validation:Optional
	Webhook *AuditLogBackendWebhook `json:"webhook,omitempty"`

	// Log defines the audit log configuration.
	// +kubebuilder:validation:Optional
	Log *AuditLogBackendLog `json:"log,omitempty"`

	// Policy defines the audit policy for the cluster.
	// If not set, an audit policy embedded in the controller is used.
	// +kubebuilder:validation:Optional
	Policy *AuditLogPolicy `json:"policy,omitempty"`
}

// AuditLogBackendWebhook defines the audit log webhook configuration.
type AuditLogBackendWebhook struct {
	// Mode defines the mode of the audit log event batching.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=batch;blocking;blocking-strict
	// +kubebuilder:default=batch
	Mode string `json:"mode,omitempty"`

	// Secret holds the kubeconfig for the audit log webhook.
	// +kubebuilder:validation:Required
	Secret *LocalObjectReference `json:"secret"`

	// InitialBackoff defines the initial backoff duration for the audit log webhook.
	// +kubebuilder:validation:Optional
	InitialBackoff metav1.Duration `json:"initialBackoff,omitempty"`

	// EventBatching defines the event batching configuration.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self.mode == 'batch' || self.eventBatching == nil",message="EventBatching can be set only if Mode is set to batch"
	EventBatching *AuditLogEventBatching `json:"eventBatching,omitempty"`
}

type AuditLogBackendLog struct {
	// Mode defines the mode of the audit log.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=batch;blocking;blocking-strict
	// +kubebuilder:default=blocking
	Mode string `json:"mode,omitempty"`

	// Path defines the path to the audit log file.
	// +kubebuilder:validation:Optional
	Path string `json:"path,omitempty"`

	// MaxAge defines the maximum age of the audit log in days.
	// +kubebuilder:validation:Optional
	MaxAge int32 `json:"maxAge,omitempty"`

	// MaxBackup defines the maximum number of audit log files to retain.
	// +kubebuilder:validation:Optional
	MaxBackup int32 `json:"maxBackup,omitempty"`

	// MaxSize defines the maximum size of the audit log file in MB.
	// +kubebuilder:validation:Optional
	MaxSize int32 `json:"maxSize,omitempty"`

	// Compress defines if the audit log file should be compressed.
	// +kubebuilder:validation:Optional
	Compress bool `json:"compress,omitempty"`

	// EventBatching defines the event batching configuration.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self.mode == 'batch' || self.eventBatching == nil",message="EventBatching can be set only if Mode is set to batch"
	EventBatching *AuditLogEventBatching `json:"eventBatching,omitempty"`
}

type AuditLogEventBatching struct {
	// BufferSize defines the number of events to buffer before batching.
	// +kubebuilder:validation:Optional
	BufferSize int32 `json:"bufferSize,omitempty"`

	// MaxSize defines the maximum number of events in one batch.
	// +kubebuilder:validation:Optional
	MaxSize int32 `json:"maxSize,omitempty"`

	// MaxWait defines the maximum amount of time to wait before unconditionally batching
	// events in the queue.
	// +kubebuilder:validation:Optional
	MaxWait int32 `json:"maxWait,omitempty"`

	// ThrottleEnable defines whether batching throttling is enabled.
	// +kubebuilder:validation:Optional
	ThrottleEnable bool `json:"throttleEnable,omitempty"`

	// ThrottleQPS defines the maximum average number of batches generated per second.
	// +kubebuilder:validation:Optional
	ThrottleQPS int32 `json:"throttleQPS,omitempty"`

	// ThrottleBurst defines the maximum number of batches generated at the same moment
	// if the allowed QPS was underutilized previously.
	// +kubebuilder:validation:Optional
	ThrottleBurst int32 `json:"throttleBurst,omitempty"`
}

type AuditLogPolicy struct {
	// ConfigMap holds the audit policy configmap.
	// The data key "policy.yaml" must contain the audit policy in YAML or JSON format.
	// +kubebuilder:validation:Required
	ConfigMap *LocalObjectReference `json:"configMap"`
}
