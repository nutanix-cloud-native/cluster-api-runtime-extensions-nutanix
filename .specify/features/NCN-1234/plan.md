<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# KubeletConfiguration Variable & Mutation Handler — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a strongly-typed `KubeletConfiguration` variable and mutation handler that
patches kubelet settings on control plane and worker nodes via strategic merge patch files.

**Architecture:** New `KubeletConfiguration` API type with 17 curated fields, enum types,
and mixed map value types. A mutation handler reads the variable from cluster-level and
node-level config, merges them (node wins), renders a `KubeletConfiguration` YAML strategic
merge patch, and injects it as a file into `KubeadmControlPlaneTemplate` /
`KubeadmConfigTemplate`. CEL rules handle cheap cross-field validations; a webhook
validator handles complex/format validations.

**Tech Stack:** Go, kubebuilder markers, controller-gen CRDs, CAPI runtime extensions,
testify, Ginkgo/Gomega.

---

## Task 1: Define KubeletConfiguration API types

**Files:**
- Create: `api/v1alpha1/kubelet_types.go`

**Step 1: Write the type file**

Create `api/v1alpha1/kubelet_types.go` with:

```go
// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologyManagerPolicy represents the policy for the Topology Manager.
type TopologyManagerPolicy string

const (
	TopologyManagerPolicyNone           TopologyManagerPolicy = "none"
	TopologyManagerPolicyBestEffort     TopologyManagerPolicy = "best-effort"
	TopologyManagerPolicyRestricted     TopologyManagerPolicy = "restricted"
	TopologyManagerPolicySingleNUMANode TopologyManagerPolicy = "single-numa-node"
)

// CPUManagerPolicy represents the policy for the CPU Manager.
type CPUManagerPolicy string

const (
	CPUManagerPolicyNone   CPUManagerPolicy = "none"
	CPUManagerPolicyStatic CPUManagerPolicy = "static"
)

// MemoryManagerPolicy represents the policy for the Memory Manager.
type MemoryManagerPolicy string

const (
	MemoryManagerPolicyNone   MemoryManagerPolicy = "None"
	MemoryManagerPolicyStatic MemoryManagerPolicy = "Static"
)

// KubeletConfiguration defines configurable fields for the kubelet's KubeletConfiguration.
// These fields are written as a strategic merge patch file applied during kubeadm init/join.
// +kubebuilder:validation:XValidation:rule="!has(self.imageGCHighThresholdPercent) || !has(self.imageGCLowThresholdPercent) || self.imageGCHighThresholdPercent > self.imageGCLowThresholdPercent",message="imageGCHighThresholdPercent must be greater than imageGCLowThresholdPercent"
// +kubebuilder:validation:XValidation:rule="!has(self.evictionSoftGracePeriod) || has(self.evictionSoft)",message="evictionSoft must be set when evictionSoftGracePeriod is set"
// +kubebuilder:validation:XValidation:rule="!has(self.evictionSoftGracePeriod) || !has(self.evictionSoft) || self.evictionSoftGracePeriod.all(k, k in self.evictionSoft)",message="evictionSoftGracePeriod keys must match evictionSoft keys"
type KubeletConfiguration struct {
	// MaxPods defines the maximum number of pods that can run on a node.
	// Default kubelet value is 110.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4096
	MaxPods *int32 `json:"maxPods,omitempty"`

	// SystemReserved is a set of ResourceName=ResourceQuantity pairs that describe
	// resources reserved for OS system daemons. Kubernetes components such as the
	// kubelet are excluded from this reservation.
	// Valid keys: cpu, memory, ephemeral-storage, pid.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=4
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['cpu', 'memory', 'ephemeral-storage', 'pid'])",message="only cpu, memory, ephemeral-storage, and pid are valid keys"
	SystemReserved map[string]resource.Quantity `json:"systemReserved,omitempty"`

	// KubeReserved is a set of ResourceName=ResourceQuantity pairs that describe
	// resources reserved for Kubernetes system components (the kubelet, container
	// runtime, etc.).
	// Valid keys: cpu, memory, ephemeral-storage, pid.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=4
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['cpu', 'memory', 'ephemeral-storage', 'pid'])",message="only cpu, memory, ephemeral-storage, and pid are valid keys"
	KubeReserved map[string]resource.Quantity `json:"kubeReserved,omitempty"`

	// EvictionHard is a map of signal names to quantities that define hard eviction
	// thresholds. When the node reaches these thresholds, pods are evicted immediately
	// with no grace period. Values may be absolute quantities (e.g. "100Mi") or
	// percentages (e.g. "10%").
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=6
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['memory.available', 'nodefs.available', 'nodefs.inodesFree', 'imagefs.available', 'imagefs.inodesFree', 'pid.available'])",message="only valid eviction signal names are allowed"
	EvictionHard map[string]string `json:"evictionHard,omitempty"`

	// EvictionSoft is a map of signal names to quantities that define soft eviction
	// thresholds. Pods are evicted after the grace period specified in
	// evictionSoftGracePeriod has elapsed. Values may be absolute quantities or
	// percentages.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=6
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['memory.available', 'nodefs.available', 'nodefs.inodesFree', 'imagefs.available', 'imagefs.inodesFree', 'pid.available'])",message="only valid eviction signal names are allowed"
	EvictionSoft map[string]string `json:"evictionSoft,omitempty"`

	// EvictionSoftGracePeriod is a map of signal names to durations that define
	// grace periods for soft eviction signals. Each key must correspond to an entry
	// in evictionSoft.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=6
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['memory.available', 'nodefs.available', 'nodefs.inodesFree', 'imagefs.available', 'imagefs.inodesFree', 'pid.available'])",message="only valid eviction signal names are allowed"
	EvictionSoftGracePeriod map[string]metav1.Duration `json:"evictionSoftGracePeriod,omitempty"`

	// ProtectKernelDefaults when enabled causes the kubelet to error if kernel flags
	// are not as it expects. Typically required by CIS benchmarks and DISA STIG.
	// +kubebuilder:validation:Optional
	ProtectKernelDefaults *bool `json:"protectKernelDefaults,omitempty"`

	// TopologyManagerPolicy controls the NUMA-aware resource alignment policy.
	// Relevant for workloads sensitive to hardware topology (GPU, HPC, telco).
	// Default kubelet value is "none".
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=none;best-effort;restricted;single-numa-node
	TopologyManagerPolicy *TopologyManagerPolicy `json:"topologyManagerPolicy,omitempty"`

	// CPUManagerPolicy controls how cpusets are assigned to containers.
	// "static" enables exclusive CPU pinning for Guaranteed QoS pods.
	// Default kubelet value is "none".
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=none;static
	CPUManagerPolicy *CPUManagerPolicy `json:"cpuManagerPolicy,omitempty"`

	// MemoryManagerPolicy controls the memory management policy on the node.
	// "Static" enables NUMA-aware memory allocation for Guaranteed QoS pods.
	// Default kubelet value is "None".
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=None;Static
	MemoryManagerPolicy *MemoryManagerPolicy `json:"memoryManagerPolicy,omitempty"`

	// PodPidsLimit is the maximum number of PIDs in any pod.
	// Prevents fork bombs and enforces per-pod PID limits.
	// Default kubelet value is -1 (unlimited).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=-1
	PodPidsLimit *int64 `json:"podPidsLimit,omitempty"`

	// ContainerLogMaxSize defines the maximum size of the container log file
	// before it is rotated. Value is a quantity (e.g. "10Mi", "256Ki").
	// Default kubelet value is "10Mi".
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^\d+(Ki|Mi|Gi)$`
	ContainerLogMaxSize *string `json:"containerLogMaxSize,omitempty"`

	// ContainerLogMaxFiles specifies the maximum number of container log files
	// that can be present for a container.
	// Default kubelet value is 5.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=2
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty"`

	// ImageGCHighThresholdPercent is the percent of disk usage after which image
	// garbage collection is always run. Must be greater than
	// imageGCLowThresholdPercent when both are set.
	// Default kubelet value is 85.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	ImageGCHighThresholdPercent *int32 `json:"imageGCHighThresholdPercent,omitempty"`

	// ImageGCLowThresholdPercent is the percent of disk usage before which image
	// garbage collection is never run. Must be less than
	// imageGCHighThresholdPercent when both are set.
	// Default kubelet value is 80.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	ImageGCLowThresholdPercent *int32 `json:"imageGCLowThresholdPercent,omitempty"`

	// MaxParallelImagePulls defines the maximum number of image pulls performed
	// in parallel by the kubelet. A value of zero means unlimited. When set to a
	// value > 0, serializeImagePulls is automatically set to false.
	// Default kubelet value is nil (serial pulls).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	MaxParallelImagePulls *int32 `json:"maxParallelImagePulls,omitempty"`

	// ShutdownGracePeriod specifies the total duration that the node should delay
	// the shutdown by for pod termination during a node shutdown.
	// Default kubelet value is "0s" (disabled).
	// +kubebuilder:validation:Optional
	ShutdownGracePeriod *metav1.Duration `json:"shutdownGracePeriod,omitempty"`

	// ShutdownGracePeriodCriticalPods specifies the duration used to terminate
	// critical pods during a node shutdown. This should be less than
	// shutdownGracePeriod.
	// Default kubelet value is "0s".
	// +kubebuilder:validation:Optional
	ShutdownGracePeriodCriticalPods *metav1.Duration `json:"shutdownGracePeriodCriticalPods,omitempty"`
}
```

**Step 2: Verify it compiles**

Run: `go build ./api/...`
Expected: SUCCESS

**Step 3: Commit**

```bash
git add api/v1alpha1/kubelet_types.go
git commit -m "feat: [NCN-1234] Add KubeletConfiguration API type with 17 strongly-typed fields"
```

---

## Task 2: Wire KubeletConfiguration into existing API hierarchy

**Files:**
- Modify: `api/v1alpha1/clusterconfig_types.go` (add field to `KubeadmClusterConfigSpec`, deprecate `MaxParallelImagePullsPerNode`)
- Modify: `api/v1alpha1/nodeconfig_types.go` (add field to `KubeadmNodeSpec`)

**Step 1: Add `KubeletConfiguration` to `KubeadmClusterConfigSpec`**

In `api/v1alpha1/clusterconfig_types.go`, add to `KubeadmClusterConfigSpec`:

```go
	// KubeletConfiguration defines kubelet settings applied to all nodes by default.
	// Per-node-group overrides can be set via controlPlane or workerConfig.
	// +kubebuilder:validation:Optional
	KubeletConfiguration *KubeletConfiguration `json:"kubeletConfiguration,omitempty"`
```

Add deprecation comment to `MaxParallelImagePullsPerNode`:

```go
	// Deprecated: Use kubeletConfiguration.maxParallelImagePulls instead.
	// This field is kept for backwards compatibility. If both this field and
	// kubeletConfiguration.maxParallelImagePulls are set, the latter takes precedence.
```

**Step 2: Add `KubeletConfiguration` to `KubeadmNodeSpec`**

In `api/v1alpha1/nodeconfig_types.go`, add to `KubeadmNodeSpec`:

```go
	// KubeletConfiguration defines kubelet settings for this node group.
	// When set, these values override the cluster-level kubeletConfiguration defaults.
	// Override semantics are per-field (not deep merge of map values within a field).
	// +kubebuilder:validation:Optional
	KubeletConfiguration *KubeletConfiguration `json:"kubeletConfiguration,omitempty"`
```

**Step 3: Verify it compiles**

Run: `go build ./api/...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add api/v1alpha1/clusterconfig_types.go api/v1alpha1/nodeconfig_types.go
git commit -m "feat: [NCN-1234] Wire KubeletConfiguration into cluster and node config specs"
```

---

## Task 3: Regenerate CRDs

**Files:**
- Modified (generated): `api/v1alpha1/crds/*.yaml`

**Step 1: Run CRD generation**

Run: `make generate-deepcopy generate-crds`
(Check Makefile for the exact target names — look for `controller-gen crd` or `generate` targets.)

**Step 2: Verify CRDs contain the new fields**

Grep the generated CRD YAMLs for `kubeletConfiguration`:

Run: `grep -l kubeletConfiguration api/v1alpha1/crds/*.yaml`
Expected: Should list the AWS, Docker, Nutanix, and Kubeadm cluster config CRDs,
plus the AWS, Docker, and Nutanix worker node config CRDs.

Verify CEL rules are present:

Run: `grep "imageGCHighThresholdPercent must be greater" api/v1alpha1/crds/*.yaml`
Expected: Should appear in all CRDs that embed KubeletConfiguration.

**Step 3: Verify it compiles**

Run: `go build ./...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add api/v1alpha1/crds/ api/v1alpha1/zz_generated.deepcopy.go
git commit -m "build: [NCN-1234] Regenerate CRDs and deepcopy for KubeletConfiguration"
```

---

## Task 4: Create the kubelet configuration patch template

**Files:**
- Create: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/embedded/kubeletconfigpatch.yaml.tmpl`

**Step 1: Write the template**

This Go template renders a KubeletConfiguration strategic merge patch. It must only
include fields that are actually set (non-nil). Follow the pattern in
`pkg/handlers/generic/mutation/kubeadm/parallelimagepulls/embedded/kubeletconfigpatch.yaml`.

```yaml
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
{{- if .MaxPods }}
maxPods: {{ .MaxPods }}
{{- end }}
{{- if .SystemReserved }}
systemReserved:
  {{- range $k, $v := .SystemReserved }}
  {{ $k }}: "{{ $v }}"
  {{- end }}
{{- end }}
{{- if .KubeReserved }}
kubeReserved:
  {{- range $k, $v := .KubeReserved }}
  {{ $k }}: "{{ $v }}"
  {{- end }}
{{- end }}
{{- if .EvictionHard }}
evictionHard:
  {{- range $k, $v := .EvictionHard }}
  {{ $k }}: "{{ $v }}"
  {{- end }}
{{- end }}
{{- if .EvictionSoft }}
evictionSoft:
  {{- range $k, $v := .EvictionSoft }}
  {{ $k }}: "{{ $v }}"
  {{- end }}
{{- end }}
{{- if .EvictionSoftGracePeriod }}
evictionSoftGracePeriod:
  {{- range $k, $v := .EvictionSoftGracePeriod }}
  {{ $k }}: "{{ $v }}"
  {{- end }}
{{- end }}
{{- if .ProtectKernelDefaults }}
protectKernelDefaults: {{ .ProtectKernelDefaults }}
{{- end }}
{{- if .TopologyManagerPolicy }}
topologyManagerPolicy: {{ .TopologyManagerPolicy }}
{{- end }}
{{- if .CPUManagerPolicy }}
cpuManagerPolicy: {{ .CPUManagerPolicy }}
{{- end }}
{{- if .MemoryManagerPolicy }}
memoryManagerPolicy: {{ .MemoryManagerPolicy }}
{{- end }}
{{- if .PodPidsLimit }}
podPidsLimit: {{ .PodPidsLimit }}
{{- end }}
{{- if .ContainerLogMaxSize }}
containerLogMaxSize: {{ .ContainerLogMaxSize }}
{{- end }}
{{- if .ContainerLogMaxFiles }}
containerLogMaxFiles: {{ .ContainerLogMaxFiles }}
{{- end }}
{{- if .ImageGCHighThresholdPercent }}
imageGCHighThresholdPercent: {{ .ImageGCHighThresholdPercent }}
{{- end }}
{{- if .ImageGCLowThresholdPercent }}
imageGCLowThresholdPercent: {{ .ImageGCLowThresholdPercent }}
{{- end }}
{{- if .MaxParallelImagePulls }}
serializeImagePulls: false
maxParallelImagePulls: {{ .MaxParallelImagePulls }}
{{- end }}
{{- if .ShutdownGracePeriod }}
shutdownGracePeriod: {{ .ShutdownGracePeriod }}
{{- end }}
{{- if .ShutdownGracePeriodCriticalPods }}
shutdownGracePeriodCriticalPods: {{ .ShutdownGracePeriodCriticalPods }}
{{- end }}
```

**Step 2: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/embedded/
git commit -m "feat: [NCN-1234] Add KubeletConfiguration strategic merge patch template"
```

---

## Task 5: Implement the mutation handler — merge logic and template rendering

**Files:**
- Create: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject.go`

**Step 1: Write the failing test**

Create: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_test.go`

Write a test that:
1. Calls `NewPatch()` to get the handler.
2. Constructs variables with `clusterConfig.kubeletConfiguration.maxPods=200`.
3. Calls `Mutate()` with a `KubeadmControlPlaneTemplate` unstructured object.
4. Asserts the result has a File at `/etc/kubernetes/patches/kubeletconfiguration+strategic.json`
   containing `maxPods: 200`.

Follow the test patterns in `pkg/handlers/generic/mutation/kubeadm/parallelimagepulls/inject_test.go`
for how to construct variables and unstructured objects.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/... -v -run TestMutate`
Expected: FAIL (handler does not exist yet)

**Step 3: Write minimal implementation**

Create `inject.go` with:
- A struct type `kubeletConfigurationPatch` holding `variableName` and `variableFieldPath`
- `NewPatch()` constructor pointing at `v1alpha1.ClusterConfigVariableName` + `"kubeletConfiguration"`
- `Mutate()` method that:
  1. Reads the variable via `variables.Get[v1alpha1.KubeletConfiguration](...)`
  2. Returns nil if not found
  3. Calls a merge function (cluster + node level)
  4. Renders the template
  5. Calls `patches.MutateIfApplicable()` for both control plane and worker selectors
  6. Appends the rendered file to `KubeadmConfigSpec.Files`

Include a `mergeKubeletConfig(clusterLevel, nodeLevel *v1alpha1.KubeletConfiguration)` function
that returns a merged config where non-nil node-level fields override cluster-level fields.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/... -v -run TestMutate`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/
git commit -m "feat: [NCN-1234] Implement KubeletConfiguration mutation handler"
```

---

## Task 6: Implement control plane and worker split handlers

**Files:**
- Create: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_controlplane.go`
- Create: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_worker.go`

Follow the pattern in `pkg/handlers/generic/mutation/kubeadm/noderegistration/inject_controlplane.go`
and `inject_worker.go` for how to create separate control-plane and worker handlers that
read from different variable paths.

The control plane handler reads from:
- Cluster default: `clusterConfig.kubeletConfiguration`
- Override: `clusterConfig.controlPlane.<provider>.kubeletConfiguration` (via `KubeadmNodeSpec`)

The worker handler reads from:
- Cluster default: `clusterConfig.kubeletConfiguration`
- Override: `workerConfig.kubeletConfiguration` (via `KubeadmNodeSpec`)

**Step 1: Write failing tests**

Test that when both cluster-level and node-level kubelet configs are set, the node-level
field wins. Test for both control plane and worker paths.

**Step 2: Run tests to verify they fail**

**Step 3: Implement the split handlers**

**Step 4: Run tests to verify they pass**

**Step 5: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/
git commit -m "feat: [NCN-1234] Add control plane and worker kubelet config handlers with merge"
```

---

## Task 7: Handle deprecated maxParallelImagePullsPerNode

**Files:**
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject.go`

**Step 1: Write failing tests**

1. Test that when only `maxParallelImagePullsPerNode=4` is set (old field), the rendered
   patch contains `maxParallelImagePulls: 4` and `serializeImagePulls: false`.
2. Test that when both fields are set, `kubeletConfiguration.maxParallelImagePulls` wins.

**Step 2: Run tests — expect FAIL**

**Step 3: Implement**

In the Mutate function, after reading the `KubeletConfiguration` variable, also read the
deprecated `maxParallelImagePullsPerNode` variable. If the new field is not set but the
deprecated one is, copy its value into `MaxParallelImagePulls`.

**Step 4: Run tests — expect PASS**

**Step 5: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/
git commit -m "feat: [NCN-1234] Handle deprecated maxParallelImagePullsPerNode with lower precedence"
```

---

## Task 8: Register the mutation handler

**Files:**
- Modify: `pkg/handlers/generic/mutation/handlers.go`

**Step 1: Add the handler to the MetaMutators list**

Import the new package and add `kubeletconfiguration.NewControlPlanePatch()` and
`kubeletconfiguration.NewWorkerPatch()` to the appropriate mutator lists.

Follow the pattern used by `parallelimagepulls.NewPatch()` in the same file.

**Step 2: Verify it compiles**

Run: `go build ./pkg/handlers/...`
Expected: SUCCESS

**Step 3: Commit**

```bash
git add pkg/handlers/generic/mutation/handlers.go
git commit -m "feat: [NCN-1234] Register KubeletConfiguration mutation handlers"
```

---

## Task 9: Add webhook validation

**Files:**
- Create: `pkg/webhook/cluster/kubeletconfiguration_validator.go`
- Modify: `pkg/webhook/cluster/validator.go` (register in MultiValidatingHandler)

**Step 1: Write failing tests**

Create: `pkg/webhook/cluster/kubeletconfiguration_validator_test.go`

1. Test: `cpuManagerPolicy=static` without CPU in `systemReserved` or `kubeReserved` → rejected.
2. Test: `cpuManagerPolicy=static` with CPU in `systemReserved` → accepted.
3. Test: `evictionHard` with value `"abc"` → rejected (invalid format).
4. Test: `evictionHard` with value `"10%"` → accepted.
5. Test: `evictionHard` with value `"100Mi"` → accepted.
6. Test: Both `maxParallelImagePullsPerNode` and `kubeletConfiguration.maxParallelImagePulls`
   set → warning emitted.

**Step 2: Run tests — expect FAIL**

**Step 3: Implement the validator**

Create a `kubeletConfigurationValidator` struct with a `validate` method that checks:
- `cpuManagerPolicy=static` requires CPU in `systemReserved` or `kubeReserved`
- `evictionHard`/`evictionSoft` values match pattern `^[0-9]+(\.[0-9]+)?(%|Ki|Mi|Gi|Ti)?$`
- Deprecated field conflict detection (warning, not rejection)

**Step 4: Register in `validator.go`**

Add the new validator to the `admission.MultiValidatingHandler()` call.

**Step 5: Run tests — expect PASS**

**Step 6: Commit**

```bash
git add pkg/webhook/cluster/kubeletconfiguration_validator.go \
       pkg/webhook/cluster/kubeletconfiguration_validator_test.go \
       pkg/webhook/cluster/validator.go
git commit -m "feat: [NCN-1234] Add webhook validation for KubeletConfiguration"
```

---

## Task 10: Add comprehensive unit tests for all 17 fields

**Files:**
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_test.go`

**Step 1: Write tests**

Add test cases for each of the 17 fields individually:
- Set one field, verify it appears in the rendered patch YAML.
- Set all fields, verify they all appear.
- Set no fields (empty struct), verify no patch file is generated.

Add merge tests:
- Cluster-level only → applied.
- Node-level only → applied.
- Both set, different fields → both fields appear (union).
- Both set, same field → node-level wins.

**Step 2: Run tests**

Run: `go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/... -v`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/
git commit -m "test: [NCN-1234] Add comprehensive unit tests for KubeletConfiguration handler"
```

---

## Task 11: Update the v4 handlers (if applicable)

**Files:**
- Check: `pkg/handlers/v4/generic/mutation/` for any handler registration that needs updating.

If v4 handlers mirror the generic handlers (check by diffing
`pkg/handlers/generic/mutation/handlers.go` with `pkg/handlers/v4/generic/mutation/handlers.go`),
add the same registration there.

**Step 1: Check if v4 handlers exist and need updating**

Run: `ls pkg/handlers/v4/generic/mutation/`

**Step 2: If needed, add registration**

**Step 3: Commit**

```bash
git add pkg/handlers/v4/
git commit -m "feat: [NCN-1234] Register KubeletConfiguration handler in v4 handlers"
```

---

## Task 12: Run full test suite and lint

**Step 1: Run linter**

Run: `make lint` (or `golangci-lint run`)
Expected: No new lint errors.

**Step 2: Run unit tests**

Run: `make test-unit` (or `go test ./...`)
Expected: ALL PASS

**Step 3: Fix any issues found**

**Step 4: Final commit if fixups needed**

```bash
git commit -m "fix: [NCN-1234] Address lint and test issues"
```
