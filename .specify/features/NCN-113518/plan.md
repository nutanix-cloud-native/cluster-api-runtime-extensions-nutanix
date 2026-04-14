<!--
 Copyright 2024 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# enforceNodeAllocatable Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in `enforceNodeAllocatable` field to the CAREN `KubeletConfiguration` API that auto-injects well-known systemd cgroup paths.

**Architecture:** New enum type + field on `KubeletConfiguration` struct, extended template rendering with sorted output and conditional cgroup path injection, extended unit tests and docs.

**Tech Stack:** Go, kubebuilder markers, Go `text/template`, Ginkgo/Gomega + standard `testing`

---

## File Map


| Action | File                                                                                               | Responsibility                                                                   |
| ------ | -------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Modify | `api/v1alpha1/kubelet_types.go`                                                                    | Add `EnforceNodeAllocatableOption` type, constants, and field                    |
| Modify | `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/kubelet_config.go`                     | Add template input field, `toTemplateInput` mapping with sort + cgroup injection |
| Modify | `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/embedded/kubeletconfigpatch.yaml.tmpl` | Render `enforceNodeAllocatable`, `systemReservedCgroup`, `kubeReservedCgroup`    |
| Modify | `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/kubelet_config_test.go`                | Unit tests for rendering                                                         |
| Modify | `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_worker_test.go`                 | Integration patch test for worker                                                |
| Modify | `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_controlplane_test.go`           | Integration patch test for control plane                                         |
| Modify | `docs/content/customization/kubeadm/kubelet-configuration.md`                                      | Document the new field                                                           |
| Run    | `make generate`                                                                                    | Regenerate deepcopy, CRD YAMLs                                                   |


---

### Task 1: Add API types

**Files:**

- Modify: `api/v1alpha1/kubelet_types.go`
- **Step 1: Add `EnforceNodeAllocatableOption` type, constants, and field**

Add after the `MemoryManagerPolicy` constants block in `api/v1alpha1/kubelet_types.go`:

```go
// EnforceNodeAllocatableOption specifies a resource type for cgroup enforcement.
type EnforceNodeAllocatableOption string

const (
	EnforceNodeAllocatablePods           EnforceNodeAllocatableOption = "pods"
	EnforceNodeAllocatableSystemReserved EnforceNodeAllocatableOption = "system-reserved"
	EnforceNodeAllocatableKubeReserved   EnforceNodeAllocatableOption = "kube-reserved"
)
```

Add the following field to the `KubeletConfiguration` struct, after `ShutdownGracePeriodCriticalPods`:

```go
	// EnforceNodeAllocatable specifies which resource types are enforced via
	// cgroups. When "system-reserved" is included, the kubelet enforces
	// systemReserved limits using the well-known systemd cgroup /system.slice.
	// When "kube-reserved" is included, the kubelet enforces kubeReserved limits
	// using /system.slice/kubelet.service. Default kubelet behaviour (when this
	// field is not set) is to enforce only pods.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=3
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Enum=pods;system-reserved;kube-reserved
	EnforceNodeAllocatable []EnforceNodeAllocatableOption `json:"enforceNodeAllocatable,omitempty"`
```

- **Step 2: Run code generation**

```bash
make generate
```

Expected: deepcopy and CRD YAML files regenerated successfully. The `IsEmpty()` method on `KubeletConfiguration` uses `equality.Semantic.DeepEqual` so it automatically handles the new field.

- **Step 3: Verify compilation**

```bash
go build ./api/...
```

Expected: compiles without errors.

- **Step 4: Commit**

```bash
git add api/
git commit -m "feat: [NCN-113518] Add enforceNodeAllocatable field to KubeletConfiguration API"
```

---

### Task 2: Extend template rendering

**Files:**

- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/kubelet_config.go`
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/embedded/kubeletconfigpatch.yaml.tmpl`
- **Step 1: Add template input field and mapping in `kubelet_config.go`**

Add to `kubeletConfigTemplateInput` struct:

```go
	EnforceNodeAllocatable  []string
	SystemReservedCgroup    string
	KubeReservedCgroup      string
```

Add import for `"slices"` and `"strings"` at the top of the file.

Add the following to `toTemplateInput` before the `return in` statement:

```go
	if len(cfg.EnforceNodeAllocatable) > 0 {
		sorted := make([]string, len(cfg.EnforceNodeAllocatable))
		for i, v := range cfg.EnforceNodeAllocatable {
			sorted[i] = string(v)
		}
		slices.Sort(sorted)
		in.EnforceNodeAllocatable = sorted
		if slices.Contains(sorted, string(v1alpha1.EnforceNodeAllocatableSystemReserved)) {
			in.SystemReservedCgroup = "/system.slice"
		}
		if slices.Contains(sorted, string(v1alpha1.EnforceNodeAllocatableKubeReserved)) {
			in.KubeReservedCgroup = "/system.slice/kubelet.service"
		}
	}
```

Remove the `"strings"` import if it was added (only `"slices"` is needed).

- **Step 2: Add template blocks in `kubeletconfigpatch.yaml.tmpl`**

Add after the `shutdownGracePeriodCriticalPods` block (at end of template):

```yaml
{{- with .EnforceNodeAllocatable }}
enforceNodeAllocatable:
  {{- range . }}
  - {{ . }}
  {{- end }}
{{- end }}
{{- with .SystemReservedCgroup }}
systemReservedCgroup: {{ . }}
{{- end }}
{{- with .KubeReservedCgroup }}
kubeReservedCgroup: {{ . }}
{{- end }}
```

- **Step 3: Verify compilation**

```bash
go build ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/...
```

Expected: compiles without errors.

- **Step 4: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/
git commit -m "feat: [NCN-113518] Render enforceNodeAllocatable with auto cgroup paths"
```

---

### Task 3: Unit tests for rendering

**Files:**

- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/kubelet_config_test.go`
- **Step 1: Write test for `enforceNodeAllocatable` with only `pods`**

Add to `kubelet_config_test.go`:

```go
func TestRenderKubeletConfigPatch_EnforceNodeAllocatable_PodsOnly(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		EnforceNodeAllocatable: []v1alpha1.EnforceNodeAllocatableOption{
			v1alpha1.EnforceNodeAllocatablePods,
		},
	})
	assert.Equal(t, []string{"pods"}, kubeletCfg.EnforceNodeAllocatable)
	assert.Empty(t, kubeletCfg.SystemReservedCgroup)
	assert.Empty(t, kubeletCfg.KubeReservedCgroup)
}
```

- **Step 2: Write test for sorted output with all three values**

```go
func TestRenderKubeletConfigPatch_EnforceNodeAllocatable_AllSorted(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		EnforceNodeAllocatable: []v1alpha1.EnforceNodeAllocatableOption{
			v1alpha1.EnforceNodeAllocatableSystemReserved,
			v1alpha1.EnforceNodeAllocatablePods,
			v1alpha1.EnforceNodeAllocatableKubeReserved,
		},
	})
	assert.Equal(t, []string{"kube-reserved", "pods", "system-reserved"}, kubeletCfg.EnforceNodeAllocatable)
	assert.Equal(t, "/system.slice", kubeletCfg.SystemReservedCgroup)
	assert.Equal(t, "/system.slice/kubelet.service", kubeletCfg.KubeReservedCgroup)
}
```

- **Step 3: Write test for `system-reserved` only (cgroup injected)**

```go
func TestRenderKubeletConfigPatch_EnforceNodeAllocatable_SystemReservedCgroup(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		EnforceNodeAllocatable: []v1alpha1.EnforceNodeAllocatableOption{
			v1alpha1.EnforceNodeAllocatableSystemReserved,
		},
	})
	assert.Equal(t, []string{"system-reserved"}, kubeletCfg.EnforceNodeAllocatable)
	assert.Equal(t, "/system.slice", kubeletCfg.SystemReservedCgroup)
	assert.Empty(t, kubeletCfg.KubeReservedCgroup)
}
```

- **Step 4: Write test for `kube-reserved` only (cgroup injected)**

```go
func TestRenderKubeletConfigPatch_EnforceNodeAllocatable_KubeReservedCgroup(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		EnforceNodeAllocatable: []v1alpha1.EnforceNodeAllocatableOption{
			v1alpha1.EnforceNodeAllocatableKubeReserved,
		},
	})
	assert.Equal(t, []string{"kube-reserved"}, kubeletCfg.EnforceNodeAllocatable)
	assert.Empty(t, kubeletCfg.SystemReservedCgroup)
	assert.Equal(t, "/system.slice/kubelet.service", kubeletCfg.KubeReservedCgroup)
}
```

- **Step 5: Write test for nil/empty field (no output)**

```go
func TestRenderKubeletConfigPatch_EnforceNodeAllocatable_Empty(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		MaxPods: ptr.To(int32(110)),
	})
	assert.Empty(t, kubeletCfg.EnforceNodeAllocatable)
	assert.Empty(t, kubeletCfg.SystemReservedCgroup)
	assert.Empty(t, kubeletCfg.KubeReservedCgroup)
}
```

- **Step 6: Run tests**

```bash
go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/... -run TestRenderKubeletConfigPatch_EnforceNodeAllocatable -v
```

Expected: all 5 tests PASS.

- **Step 7: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/kubelet_config_test.go
git commit -m "test: [NCN-113518] Unit tests for enforceNodeAllocatable rendering"
```

---

### Task 4: Integration patch tests

**Files:**

- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_worker_test.go`
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_controlplane_test.go`
- **Step 1: Add worker patch test case**

Add a new entry to `testDefs` in `inject_worker_test.go`:

```go
		{
			Name: "kubeletConfiguration with enforceNodeAllocatable set at worker override",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.KubeletConfiguration{
						EnforceNodeAllocatable: []v1alpha1.EnforceNodeAllocatableOption{
							v1alpha1.EnforceNodeAllocatableSystemReserved,
							v1alpha1.EnforceNodeAllocatablePods,
							v1alpha1.EnforceNodeAllocatableKubeReserved,
						},
					},
					VariableName,
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.And(
						gomega.HaveKeyWithValue(
							"path",
							kubeletConfigurationPatchFilePath,
						),
						gomega.HaveKeyWithValue(
							"content",
							gomega.And(
								gomega.ContainSubstring("enforceNodeAllocatable:"),
								gomega.ContainSubstring("- kube-reserved"),
								gomega.ContainSubstring("- pods"),
								gomega.ContainSubstring("- system-reserved"),
								gomega.ContainSubstring("systemReservedCgroup: /system.slice"),
								gomega.ContainSubstring("kubeReservedCgroup: /system.slice/kubelet.service"),
							),
						),
					),
				),
			}},
		},
```

- **Step 2: Add control plane patch test case**

Add a similar test case to `inject_controlplane_test.go` following its existing pattern (uses `v1alpha1.ClusterConfigVariableName` with field path `v1alpha1.ControlPlaneConfigVariableName, VariableName` and `request.NewKubeadmControlPlaneTemplateRequestItem`).

- **Step 3: Run integration tests**

```bash
go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/... -v
```

Expected: all tests PASS.

- **Step 4: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/
git commit -m "test: [NCN-113518] Integration patch tests for enforceNodeAllocatable"
```

---

### Task 5: Documentation

**Files:**

- Modify: `docs/content/customization/kubeadm/kubelet-configuration.md`
- **Step 1: Add `enforceNodeAllocatable` to the supported options list and add a new section**

Add `enforceNodeAllocatable` to the existing bullet list of supported options.

Add a new section after the existing examples:

```markdown
## Enforce node allocatable

The `enforceNodeAllocatable` field controls which resource reservations are enforced via
cgroups. Accepted values are `pods`, `system-reserved`, and `kube-reserved`.

When `system-reserved` is included, CAREN automatically configures the well-known systemd
cgroup path `/system.slice` for enforcement. When `kube-reserved` is included, CAREN
configures `/system.slice/kubelet.service`. You do not need to specify cgroup paths.

This field is optional. When not set, the kubelet default behaviour (`pods` only) applies
and no changes are made to existing clusters.

### Example: enforce system and kube reservations

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          controlPlane:
            kubeletConfiguration:
              systemReserved:
                cpu: "500m"
                memory: "1Gi"
              kubeReserved:
                cpu: "200m"
                memory: "512Mi"
              enforceNodeAllocatable:
                - pods
                - system-reserved
                - kube-reserved
```

```

- [ ] **Step 2: Commit**

```bash
git add docs/content/customization/kubeadm/kubelet-configuration.md
git commit -m "docs: [NCN-113518] Document enforceNodeAllocatable field"
```

---

### Task 6: Final verification

- **Step 1: Run full test suite for the package**

```bash
go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/... -v
```

Expected: all tests PASS.

- **Step 2: Run linter**

```bash
make lint
```

Expected: no new linter errors.

- **Step 3: Verify CRD YAMLs contain the new field**

Check that `api/v1alpha1/crds/caren.nutanix.com_awsworkernodeconfigs.yaml` (and similar CRD files) contain `enforceNodeAllocatable` with the enum constraint.

- **Step 4: Final commit if any generated files changed**

```bash
git add -A
git status
# If there are changes:
git commit -m "build: [NCN-113518] Regenerate CRDs and deepcopy"
```
