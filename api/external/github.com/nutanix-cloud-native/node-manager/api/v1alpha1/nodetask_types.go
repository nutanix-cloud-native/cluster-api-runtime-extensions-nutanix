// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeSelector defines node selection criteria for a NodeTask.
// If omitted or empty, the NodeTask applies to all nodes in the cluster.
//
// +kubebuilder:validation:XValidation:rule="(has(self.matchLabels) && size(self.matchLabels) > 0) || (has(self.matchExpressions) && size(self.matchExpressions) > 0) || (has(self.nodeNames) && size(self.nodeNames) > 0)",message="nodeSelector must specify at least one of matchLabels, matchExpressions, or nodeNames"
// +kubebuilder:validation:XValidation:rule="!((has(self.nodeNames) && size(self.nodeNames) > 0) && ((has(self.matchLabels) && size(self.matchLabels) > 0) || (has(self.matchExpressions) && size(self.matchExpressions) > 0)))",message="nodeNames cannot be combined with matchLabels or matchExpressions"
type NodeSelector struct {
	// matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
	// map is equivalent to an element of matchExpressions, whose key field is "key",
	// the operator is "In", and the values array contains only "value".
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// matchExpressions is a list of label selector requirements. The requirements are ANDed.
	// +optional
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`

	// nodeNames is a list of specific node names to target.
	// Cannot be combined with matchLabels or matchExpressions.
	// +optional
	NodeNames []string `json:"nodeNames,omitempty"`
}

// Action represents a single action to perform in a NodeTask.
// It uses a discriminated union pattern where the `type` field determines
// which fields are valid and required.
// Supported types: "WriteFile", "ExecuteCommand".
type Action struct {
	// type specifies the action type. Determines which fields below are used.
	// +kubebuilder:validation:Enum=WriteFile;ExecuteCommand
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// name is a unique name for this action within the NodeTask.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// files is a list of files to write, processed sequentially in order.
	// Required when type == "WriteFile".
	// +kubebuilder:validation:MinItems=1
	// +optional
	Files []FileSpec `json:"files,omitempty"`

	// commands is a list of command strings to run on the host via chroot (sh -c per command).
	// Required when type == "ExecuteCommand"; each string must be non-empty after trimming whitespace.
	// +kubebuilder:validation:MinItems=1
	// +listType=atomic
	// +optional
	Commands []string `json:"commands,omitempty"`

	// timeout limits the duration of the entire action (e.g. all commands). When exceeded, the action is aborted and marked Failed.
	// Used when type == "ExecuteCommand".
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// ConfigMapRef references a key in a ConfigMap in the same namespace as the NodeTask.
type ConfigMapRef struct {
	// name is the name of the ConfigMap.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// key is the key in ConfigMap.Data to read.
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// SecretRef references a key in a Secret in the same namespace as the NodeTask.
type SecretRef struct {
	// name is the name of the Secret.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// key is the key in Secret.Data to read (value is base64-decoded when read).
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// ContentFrom references file content from a ConfigMap or Secret. Exactly one of configMap or secret must be set.
// +kubebuilder:validation:XValidation:rule="(has(self.configMap) && !has(self.secret)) || (!has(self.configMap) && has(self.secret))",message="exactly one of configMap or secret must be set"
type ContentFrom struct {
	// configMap references a ConfigMap (name + key) in the same namespace.
	// +optional
	ConfigMapRef *ConfigMapRef `json:"configMap,omitempty"`
	// secret references a Secret (name + key) in the same namespace.
	// +optional
	SecretRef *SecretRef `json:"secret,omitempty"`
}

// FileSpec defines a single file to write.
// Exactly one of content or contentFrom must be set.
// +kubebuilder:validation:XValidation:rule="(has(self.content) && !has(self.contentFrom)) || (!has(self.content) && has(self.contentFrom))",message="exactly one of content or contentFrom must be set"
type FileSpec struct {
	// path is the absolute file path on the host filesystem.
	// Must start with "/" and be validated for path traversal attempts.
	// +kubebuilder:validation:Pattern=^/.*$
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// content is the file content as inline text. Omit when using contentFrom.
	// Maximum size: 1 MiB (1,048,576 bytes).
	// +kubebuilder:validation:MaxLength=1048576
	// +optional
	Content string `json:"content,omitempty"`

	// contentFrom sources file content from a ConfigMap or Secret in the same namespace. Omit when using content.
	// +optional
	ContentFrom *ContentFrom `json:"contentFrom,omitempty"`

	// permissions is the file permissions in octal format (e.g., "0644", "0755").
	// Must match pattern ^0[0-7]{3}$.
	// +kubebuilder:validation:Pattern="^0[0-7]{3}$"
	// +kubebuilder:validation:Required
	Permissions string `json:"permissions"`

	// owner is the file ownership in "user:group" or "uid:gid" format.
	// Must match pattern ^[^:]+:[^:]+$.
	// +kubebuilder:validation:Pattern="^[^:]+:[^:]+$"
	// +kubebuilder:validation:Required
	Owner string `json:"owner"`
}

// NodeTaskSpec defines the desired state of NodeTask.
type NodeTaskSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// nodeSelector defines node selection criteria. If omitted or empty, applies to all nodes.
	// +optional
	NodeSelector *NodeSelector `json:"nodeSelector,omitempty"`

	// actions is a list of actions to perform, executed sequentially in order.
	// Currently only "WriteFile" action type is supported.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Actions []Action `json:"actions"`
}

// NodeTaskPhase represents the phase of a NodeTask or node status.
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;PartiallyFailed
type NodeTaskPhase string

const (
	// NodeTaskPhasePending indicates no nodes have started execution yet.
	NodeTaskPhasePending NodeTaskPhase = "Pending"
	// NodeTaskPhaseRunning indicates at least one node is currently executing.
	NodeTaskPhaseRunning NodeTaskPhase = "Running"
	// NodeTaskPhaseSucceeded indicates all nodes have succeeded.
	NodeTaskPhaseSucceeded NodeTaskPhase = "Succeeded"
	// NodeTaskPhaseFailed indicates all nodes have failed.
	NodeTaskPhaseFailed NodeTaskPhase = "Failed"
	// NodeTaskPhasePartiallyFailed indicates some nodes succeeded and some failed.
	NodeTaskPhasePartiallyFailed NodeTaskPhase = "PartiallyFailed"
)

// ActionStatus represents the status of an action within a node.
type ActionStatus struct {
	// name is the action name matching the spec.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// phase is the action phase.
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
	// +kubebuilder:validation:Required
	Phase NodeTaskPhase `json:"phase"`

	// message is a success message if phase is Succeeded.
	// +optional
	Message string `json:"message,omitempty"`

	// error is the error message if phase is Failed.
	// +optional
	Error string `json:"error,omitempty"`
}

// NodeStatus represents the status of a NodeTask on a specific node.
type NodeStatus struct {
	// nodeName is the name of the node.
	// +kubebuilder:validation:Required
	NodeName string `json:"nodeName"`

	// phase is the node-specific phase.
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
	// +kubebuilder:validation:Required
	Phase NodeTaskPhase `json:"phase"`

	// startTime is when execution started on this node.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// completionTime is when execution completed on this node.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// error is the error message if phase is Failed.
	// +optional
	Error string `json:"error,omitempty"`

	// actions is the per-action status within this node.
	// +kubebuilder:validation:Required
	Actions []ActionStatus `json:"actions"`
}

// NodeTaskSummary represents aggregated statistics across all nodes.
type NodeTaskSummary struct {
	// totalNodes is the total number of nodes matching nodeSelector.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	TotalNodes int32 `json:"totalNodes"`

	// succeededNodes is the number of nodes with phase Succeeded.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	SucceededNodes int32 `json:"succeededNodes"`

	// failedNodes is the number of nodes with phase Failed.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	FailedNodes int32 `json:"failedNodes"`

	// pendingNodes is the number of nodes with phase Pending.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	PendingNodes int32 `json:"pendingNodes"`

	// runningNodes is the number of nodes with phase Running.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	RunningNodes int32 `json:"runningNodes"`
}

// NodeTaskStatus defines the observed state of NodeTask.
type NodeTaskStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/
	// sig-architecture/api-conventions.md#typical-status-properties

	// phase is the overall task phase, calculated from node statuses.
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;PartiallyFailed
	// +kubebuilder:validation:Required
	Phase NodeTaskPhase `json:"phase"`

	// nodeStatuses is an array of per-node status entries.
	// Each reconciler updates only its own node's entry.
	// +kubebuilder:validation:Required
	NodeStatuses []NodeStatus `json:"nodeStatuses"`

	// summary is aggregated statistics across all nodes.
	// +kubebuilder:validation:Required
	Summary NodeTaskSummary `json:"summary"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`,description="Overall phase of the NodeTask"
// +kubebuilder:printcolumn:name="Succeeded",type=integer,JSONPath=`.status.summary.succeededNodes`,description="Number of nodes that succeeded"
// +kubebuilder:printcolumn:name="Failed",type=integer,JSONPath=`.status.summary.failedNodes`,description="Number of nodes that failed"
// +kubebuilder:printcolumn:name="Total",type=integer,JSONPath=`.status.summary.totalNodes`,description="Total number of targeted nodes"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`,description="Age of the NodeTask"

// NodeTask is the Schema for the nodetasks API.
type NodeTask struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of NodeTask
	// +required
	Spec NodeTaskSpec `json:"spec"`

	// status defines the observed state of NodeTask
	// +optional
	Status NodeTaskStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// NodeTaskList contains a list of NodeTask.
type NodeTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []NodeTask `json:"items"`
}

//nolint:gochecknoinits // init function required by kubebuilder for scheme registration
func init() {
	SchemeBuilder.Register(&NodeTask{}, &NodeTaskList{})
}
