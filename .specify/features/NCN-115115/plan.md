<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Automatic kubelet resource reservations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in `automaticReservations` setting to `kubeletConfiguration` that makes
each node compute `kubeReserved` and a hard-eviction threshold at boot, scaled to the node's
actual CPU/memory, mutually exclusive with explicit reservation values.

**Architecture:** A new CAREN directive (`automaticReservations.profile: CapacityTiered`) on the
existing `KubeletConfiguration` API type. When set, the existing kubelet-configuration mutation
handler injects an embedded POSIX shell script plus a `preKubeadmCommand` into the
KubeadmConfigTemplate / KubeadmControlPlaneTemplate. The script reads node capacity and writes a
strategic-merge kubelet patch into the already-wired `/etc/kubernetes/patches/` directory before
kubeadm runs. Mutual exclusivity is enforced declaratively (CEL marker) and via the existing
admission webhook for a clear message. Opt-in and a byte-identical no-op when unset, so no handler
version bump and no rollouts on upgrade.

**Tech Stack:** Go, kubebuilder/controller-gen markers + CEL, CAPI topology mutation handlers,
POSIX `sh`, Ginkgo/Gomega (handler + webhook tests), testify (render + script tests).

**Spec:** [./spec.md](./spec.md)

---

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. API-First | Pass | New types live in `api/v1alpha1`; no business logic added to API types. |
| II. Handler-per-Provider | Pass | Change is in the provider-agnostic `generic` kubelet handler; works for all providers. |
| III. Library-First | Pass | Logic stays in `pkg/handlers/...`; `cmd/` untouched. |
| IV. Tests Required | Pass | Script test (testify), handler tests (Ginkgo), webhook test (Ginkgo). |
| V. Code Style | Pass | Import aliases per repo; no narrating comments; shellcheck/shfmt clean. |
| VI. Dependency Management | Pass | No new Go deps. Script uses coreutils/awk already present on nodes. |
| VII. Handler Version Safety | Pass | Opt-in; `GeneratePatches` output unchanged for inputs without the field. Verified in T7. No version bump. |
| VIII. Handler Documentation | Pass | `docs/content/customization/kubeadm/kubelet-configuration.md` updated (T8). |

## File Structure

```text
api/v1alpha1/
├── kubelet_types.go                  # MODIFY: add AutomaticReservations type, field, CEL marker
├── kubelet_types_test.go             # MODIFY: IsEmpty covers AutomaticReservations
└── zz_generated.deepcopy.go          # GENERATED: make generate

pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/
├── automaticreservations.go          # CREATE: embed script, enabled() helper, file/command builders
├── embedded/compute-reservations.sh  # CREATE: POSIX sh that computes + writes the patch
├── automaticreservations_script_test.go # CREATE: runs the script with overrides, asserts output
├── kubelet_config.go                 # MODIFY: hasRenderableKubeletFields helper
├── inject_worker.go                  # MODIFY: inject script + preKubeadmCommand; guard patch file
├── inject_worker_test.go             # MODIFY: cases for auto worker
├── inject_controlplane.go            # MODIFY: same as worker for KCP template
└── inject_controlplane_test.go       # MODIFY: cases for auto control plane

pkg/webhook/cluster/
├── kubeletconfiguration_validator.go      # MODIFY: reject auto + explicit reservation combo
└── kubeletconfiguration_validator_test.go # MODIFY: accept/reject cases

api/v1alpha1/crds/*.yaml               # GENERATED: make generate
docs/content/customization/kubeadm/kubelet-configuration.md # MODIFY: document feature
```

**Structure Decision:** Reuse the existing kubelet-configuration handler package rather than
creating a new handler — the directive is part of `kubeletConfiguration` and shares the same
selectors, variables, and patch directory. The boot script is the only new "unit": a single
embedded asset with one responsibility (capacity → patch file), testable in isolation.

## Key design facts (read before coding)

- The default ClusterClasses already set `patches.directory: /etc/kubernetes/patches` for both
  init and join, and `preKubeadmCommands` run **before** kubeadm. A script that writes a patch
  file there is consumed automatically. (Confirmed in
  `charts/.../defaultclusterclasses/*-cluster-class.yaml`.)
- The existing handler writes YAML content into a `...+strategic.json` file
  (`kubelet_config.go` + `embedded/kubeletconfigpatch.yaml.tmpl`). The boot script MUST follow
  the **same** convention (YAML body, `...+strategic.json` filename) for consistency.
- `KubeletConfiguration.IsEmpty()` uses `equality.Semantic.DeepEqual` against an empty struct, so
  a non-nil `AutomaticReservations` automatically makes the config non-empty — no `IsEmpty`
  change needed, but the handler must NOT render a near-empty kubelet patch when only
  `automaticReservations` is set.
- `automaticReservations` is a CAREN directive, NOT a kubelet field; it is intentionally absent
  from `kubeletconfigpatch.yaml.tmpl` and must never be rendered into the kubelet patch.
- Mutating both Files and PreKubeadmCommands on the templates is an established pattern
  (`pkg/handlers/generic/mutation/generic/kubeproxymode/inject.go`).

## Tasks

Atomic, TDD-ordered. Run all Go commands through devbox: prefix with `devbox run --`.

### Task 1: API type — `AutomaticReservations` + CEL mutual exclusivity

**Files:**

- Modify: `api/v1alpha1/kubelet_types.go`
- Test: `api/v1alpha1/kubelet_types_test.go`

- [ ] **Step 1: Write the failing test**

Add to `api/v1alpha1/kubelet_types_test.go`:

```go
func TestKubeletConfiguration_IsEmpty_AutomaticReservations(t *testing.T) {
	cfg := &KubeletConfiguration{
		AutomaticReservations: &AutomaticReservations{
			Profile: ReservationProfileCapacityTiered,
		},
	}
	assert.False(t, cfg.IsEmpty())
}
```

(Use the import alias and `assert` already present in that test file; if absent, add
`"github.com/stretchr/testify/assert"`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `devbox run -- go test ./api/v1alpha1/ -run TestKubeletConfiguration_IsEmpty_AutomaticReservations`
Expected: FAIL — `ReservationProfileCapacityTiered`/`AutomaticReservations` undefined (compile error).

- [ ] **Step 3: Add the types and field**

In `api/v1alpha1/kubelet_types.go`, add above `KubeletConfiguration`:

```go
// ReservationProfile selects an automatic resource reservation strategy.
type ReservationProfile string

const (
	// ReservationProfileCapacityTiered reserves tiered percentages of the node's
	// total CPU and memory capacity: smaller percentages as the node gets larger.
	ReservationProfileCapacityTiered ReservationProfile = "CapacityTiered"
)

// AutomaticReservations enables computing node resource reservations at boot from
// the node's actual CPU and memory capacity, instead of specifying explicit values.
type AutomaticReservations struct {
	// Profile selects the reservation strategy.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=CapacityTiered
	Profile ReservationProfile `json:"profile"`
}
```

Add a CEL marker to the existing `KubeletConfiguration` doc-comment marker block (alongside the
current `XValidation` rules, before `type KubeletConfiguration struct {`):

```go
// +kubebuilder:validation:XValidation:rule="!has(self.automaticReservations) || (!has(self.systemReserved) && !has(self.kubeReserved) && !has(self.evictionHard))",message="automaticReservations cannot be combined with systemReserved, kubeReserved, or evictionHard"
```

Add the field inside `KubeletConfiguration` (place it directly above `SystemReserved`):

```go
	// AutomaticReservations, when set, makes each node compute its kubeReserved
	// resources and a hard eviction threshold at boot from its actual CPU and
	// memory capacity. Mutually exclusive with systemReserved, kubeReserved, and
	// evictionHard.
	// +kubebuilder:validation:Optional
	AutomaticReservations *AutomaticReservations `json:"automaticReservations,omitempty"`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `devbox run -- go test ./api/v1alpha1/ -run TestKubeletConfiguration_IsEmpty_AutomaticReservations`
Expected: PASS.

- [ ] **Step 5: Regenerate deepcopy + CRDs + variable schemas**

Run: `devbox run -- make generate`
Expected: updates `api/v1alpha1/zz_generated.deepcopy.go` (adds `AutomaticReservations` DeepCopy)
and `api/v1alpha1/crds/*.yaml` (adds the new property + the new `x-kubernetes-validations` rule).
The `charts/.../defaultclusterclasses/*.yaml` files MUST NOT change (the field is not used by
defaults) — confirm with `git status`.

- [ ] **Step 6: Commit**

```bash
git add api/v1alpha1/kubelet_types.go api/v1alpha1/kubelet_types_test.go \
        api/v1alpha1/zz_generated.deepcopy.go api/v1alpha1/crds
git commit -m "feat: [NCN-115115] Add automaticReservations kubelet API field"
```

### Task 2: Boot script that computes reservations

**Files:**

- Create: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/embedded/compute-reservations.sh`
- Create: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/automaticreservations.go`
  (only the `//go:embed` var is needed for this task; the rest is added in Task 3)
- Test: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/automaticreservations_script_test.go`

- [ ] **Step 1: Write the failing test**

Create `automaticreservations_script_test.go`:

```go
// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"sigs.k8s.io/yaml"
)

func TestComputeReservationsScript(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "compute-reservations.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(computeReservationsScript), 0o755))

	tests := []struct {
		name       string
		cores      int
		memKiB     int
		wantCPU    string
		wantMemory string
	}{
		{"1 core 512Mi", 1, 512 * 1024, "60m", "255Mi"},
		{"2 cores 8Gi", 2, 8 * 1024 * 1024, "70m", "1843Mi"},
		{"8 cores 16Gi", 8, 16 * 1024 * 1024, "90m", "2662Mi"},
		{"16 cores 64Gi", 16, 64 * 1024 * 1024, "110m", "5611Mi"},
		{"128 cores 512Gi", 128, 512 * 1024 * 1024, "390m", "17407Mi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outDir := t.TempDir()
			cmd := exec.Command("/bin/sh", scriptPath)
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("CAREN_NODE_CPU_CORES=%d", tt.cores),
				fmt.Sprintf("CAREN_NODE_MEMORY_KIB=%d", tt.memKiB),
				"CAREN_KUBELET_PATCH_DIR="+outDir,
			)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))

			content, err := os.ReadFile(
				filepath.Join(outDir, "kubeletconfiguration50+strategic.json"),
			)
			require.NoError(t, err)

			var kc kubeletconfigv1beta1.KubeletConfiguration
			require.NoError(t, yaml.Unmarshal(content, &kc))
			assert.Equal(t, tt.wantCPU, kc.KubeReserved["cpu"])
			assert.Equal(t, tt.wantMemory, kc.KubeReserved["memory"])
			assert.Equal(t, "100Mi", kc.EvictionHard["memory.available"])
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `devbox run -- go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/ -run TestComputeReservationsScript`
Expected: FAIL — `computeReservationsScript` undefined (compile error).

- [ ] **Step 3: Create the script and the embed var**

Create `embedded/compute-reservations.sh`:

```sh
#!/bin/sh
# Computes kubelet kubeReserved (CPU, memory) and a hard eviction threshold from
# the node's actual capacity, then writes a strategic-merge kubelet patch that
# kubeadm consumes. Inputs can be overridden via environment for testing.
set -eu

patch_dir="${CAREN_KUBELET_PATCH_DIR:-/etc/kubernetes/patches}"
patch_file="${patch_dir}/kubeletconfiguration50+strategic.json"

cores="${CAREN_NODE_CPU_CORES:-$(grep -c '^processor' /proc/cpuinfo)}"
mem_kib="${CAREN_NODE_MEMORY_KIB:-$(awk '/^MemTotal:/ {print $2}' /proc/meminfo)}"

if ! [ "${cores}" -ge 1 ] 2>/dev/null; then
  echo "compute-reservations: could not determine CPU core count" >&2
  exit 1
fi
if ! [ "${mem_kib}" -ge 1 ] 2>/dev/null; then
  echo "compute-reservations: could not determine total memory" >&2
  exit 1
fi

# CPU reservation in millicores. Tiered: 6% of the 1st core, 1% of the 2nd,
# 0.5% of cores 3-4, 0.25% beyond 4. Computed in tenths of a millicore to keep
# integer arithmetic, then rounded to the nearest millicore.
tenths=0
if [ "${cores}" -ge 1 ]; then tenths=$((tenths + 600)); fi
if [ "${cores}" -ge 2 ]; then tenths=$((tenths + 100)); fi
n34=$((cores - 2))
if [ "${n34}" -lt 0 ]; then n34=0; fi
if [ "${n34}" -gt 2 ]; then n34=2; fi
tenths=$((tenths + n34 * 50))
n5=$((cores - 4))
if [ "${n5}" -lt 0 ]; then n5=0; fi
tenths=$((tenths + n5 * 25))
cpu_milli=$(((tenths + 5) / 10))

# Memory reservation in MiB. Tiers use 1Gi = 1024 MiB boundaries:
# 255Mi below 1Gi; else 25% of first 4Gi + 20% of next 4Gi + 10% of next 8Gi
# + 6% of next 112Gi + 2% above 128Gi. Per-tier floor (integer division).
total_mib=$((mem_kib / 1024))
if [ "${total_mib}" -lt 1024 ]; then
  mem_mib=255
else
  mem_mib=0
  t=${total_mib}
  if [ "${t}" -gt 4096 ]; then t=4096; fi
  mem_mib=$((mem_mib + t * 25 / 100))
  t=$((total_mib - 4096))
  if [ "${t}" -lt 0 ]; then t=0; fi
  if [ "${t}" -gt 4096 ]; then t=4096; fi
  mem_mib=$((mem_mib + t * 20 / 100))
  t=$((total_mib - 8192))
  if [ "${t}" -lt 0 ]; then t=0; fi
  if [ "${t}" -gt 8192 ]; then t=8192; fi
  mem_mib=$((mem_mib + t * 10 / 100))
  t=$((total_mib - 16384))
  if [ "${t}" -lt 0 ]; then t=0; fi
  if [ "${t}" -gt 114688 ]; then t=114688; fi
  mem_mib=$((mem_mib + t * 6 / 100))
  t=$((total_mib - 131072))
  if [ "${t}" -lt 0 ]; then t=0; fi
  mem_mib=$((mem_mib + t * 2 / 100))
fi

mkdir -p "${patch_dir}"
cat >"${patch_file}" <<EOF
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
kubeReserved:
  cpu: "${cpu_milli}m"
  memory: "${mem_mib}Mi"
evictionHard:
  memory.available: "100Mi"
EOF
```

Create `automaticreservations.go` with just the embed for now:

```go
// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import _ "embed"

//go:embed embedded/compute-reservations.sh
var computeReservationsScript string
```

- [ ] **Step 4: Run test to verify it passes**

Run: `devbox run -- go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/ -run TestComputeReservationsScript -v`
Expected: PASS for all five sub-tests.

- [ ] **Step 5: Lint the script**

Run: `devbox run -- shellcheck pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/embedded/compute-reservations.sh`
and `devbox run -- shfmt -d pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/embedded/compute-reservations.sh`
Expected: no findings (shfmt prints no diff). Apply `shfmt -w` if it reports formatting.

- [ ] **Step 6: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/embedded/compute-reservations.sh \
        pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/automaticreservations.go \
        pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/automaticreservations_script_test.go
git commit -m "feat: [NCN-115115] Add boot-time kubelet reservation script"
```

### Task 3: Wire the script + preKubeadmCommand into the handler

**Files:**

- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/automaticreservations.go`
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/kubelet_config.go`
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_worker.go`
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_controlplane.go`
- Test: `inject_worker_test.go`, `inject_controlplane_test.go`

- [ ] **Step 1: Write the failing tests**

Add to the `testDefs` slice in `inject_worker_test.go`:

```go
{
	Name: "automaticReservations at worker injects script and preKubeadmCommand",
	Vars: []runtimehooksv1.Variable{
		capitest.VariableWithValue(
			v1alpha1.WorkerConfigVariableName,
			v1alpha1.KubeletConfiguration{
				AutomaticReservations: &v1alpha1.AutomaticReservations{
					Profile: v1alpha1.ReservationProfileCapacityTiered,
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
	ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
		{
			Operation: "add",
			Path:      "/spec/template/spec/files",
			ValueMatcher: gomega.ContainElement(
				gomega.HaveKeyWithValue("path", computeReservationsScriptPath),
			),
		},
		{
			Operation:    "add",
			Path:         "/spec/template/spec/preKubeadmCommands",
			ValueMatcher: gomega.ContainElement(computeReservationsCommand),
		},
	},
},
```

Add the equivalent case to `inject_controlplane_test.go`, using
`v1alpha1.ClusterConfigVariableName` with field path `controlPlane`→`kubeletConfiguration`
(mirror the existing control-plane cases in that file), `RequestItem:
request.NewKubeadmControlPlaneTemplateRequestItem("")`, and patch paths
`/spec/template/spec/kubeadmConfigSpec/files` and
`/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `devbox run -- go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/`
Expected: FAIL — `computeReservationsScriptPath`/`computeReservationsCommand` undefined.

- [ ] **Step 3: Add helpers in `automaticreservations.go`**

Replace the file body with:

```go
// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	_ "embed"

	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const (
	computeReservationsScriptPath = "/etc/caren/compute-kubelet-reservations.sh"
	computeReservationsCommand    = "/bin/sh " + computeReservationsScriptPath
)

//go:embed embedded/compute-reservations.sh
var computeReservationsScript string

func automaticReservationsEnabled(cfg *v1alpha1.KubeletConfiguration) bool {
	return cfg != nil && cfg.AutomaticReservations != nil
}

func computeReservationsScriptFile() bootstrapv1.File {
	return bootstrapv1.File{
		Path:        computeReservationsScriptPath,
		Owner:       "root:root",
		Permissions: "0755",
		Content:     computeReservationsScript,
	}
}
```

- [ ] **Step 4: Add `hasRenderableKubeletFields` in `kubelet_config.go`**

Append to `kubelet_config.go`:

```go
// hasRenderableKubeletFields reports whether cfg has any kubelet fields to render
// into a patch, ignoring the AutomaticReservations directive (which is not a
// kubelet field and is handled by the boot-time script instead).
func hasRenderableKubeletFields(cfg *v1alpha1.KubeletConfiguration) bool {
	if cfg == nil {
		return false
	}
	withoutAuto := *cfg
	withoutAuto.AutomaticReservations = nil
	return !withoutAuto.IsEmpty()
}
```

- [ ] **Step 5: Update `inject_worker.go` Mutate**

Replace the render + closure section (from the `kubeletConfigPatch, err := renderKubeletConfigPatch(finalCfg)` line through the end of `MutateIfApplicable`) with:

```go
	var kubeletConfigPatch *bootstrapv1.File
	if hasRenderableKubeletFields(finalCfg) {
		kubeletConfigPatch, err = renderKubeletConfigPatch(finalCfg)
		if err != nil {
			return err
		}
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.WorkersKubeadmConfigTemplateSelector(),
		log,
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding KubeletConfiguration patch to worker node kubeadm config template")

			spec := &obj.Spec.Template.Spec
			if kubeletConfigPatch != nil {
				spec.Files = append(spec.Files, *kubeletConfigPatch)
			}
			if automaticReservationsEnabled(finalCfg) {
				spec.Files = append(spec.Files, computeReservationsScriptFile())
				spec.PreKubeadmCommands = append(
					spec.PreKubeadmCommands,
					computeReservationsCommand,
				)
			}

			return nil
		},
	)
```

- [ ] **Step 6: Update `inject_controlplane.go` Mutate**

Apply the same change, but the closure targets `*controlplanev1.KubeadmControlPlaneTemplate`
and uses `spec := &obj.Spec.Template.Spec.KubeadmConfigSpec` (so `spec.Files` and
`spec.PreKubeadmCommands` resolve to the KubeadmConfigSpec fields), with the existing
`selectors.ControlPlane()` selector and control-plane log message.

- [ ] **Step 7: Run tests to verify they pass**

Run: `devbox run -- go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/`
Expected: PASS, including the existing render/no-op cases.

- [ ] **Step 8: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration
git commit -m "feat: [NCN-115115] Inject reservation script via preKubeadmCommand"
```

### Task 4: Handler injects nothing extra when auto is unset (no-op guard)

**Files:**

- Test: `inject_worker_test.go`

- [ ] **Step 1: Write the test**

Add a case asserting that a worker config WITHOUT `automaticReservations` (e.g. the existing
`maxPods: 110` case already present) produces NO `preKubeadmCommands` patch and NO file with
`path == computeReservationsScriptPath`. Add a dedicated assertion case:

```go
{
	Name: "maxPods only does not inject reservation script",
	Vars: []runtimehooksv1.Variable{
		capitest.VariableWithValue(
			v1alpha1.WorkerConfigVariableName,
			v1alpha1.KubeletConfiguration{MaxPods: ptr.To(int32(110))},
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
		ValueMatcher: gomega.Not(gomega.ContainElement(
			gomega.HaveKeyWithValue("path", computeReservationsScriptPath),
		)),
	}},
},
```

- [ ] **Step 2: Run and verify it passes**

Run: `devbox run -- go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/ -v`
Expected: PASS (the guard added in Task 3 already produces this behaviour).

- [ ] **Step 3: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_worker_test.go
git commit -m "test: [NCN-115115] Assert no script injected without automaticReservations"
```

### Task 5: Webhook mutual-exclusivity check

**Files:**

- Modify: `pkg/webhook/cluster/kubeletconfiguration_validator.go`
- Test: `pkg/webhook/cluster/kubeletconfiguration_validator_test.go`

- [ ] **Step 1: Write the failing tests**

Add two `Context` blocks mirroring the existing style in
`kubeletconfiguration_validator_test.go`:

```go
Context("automaticReservations combined with kubeReserved", func() {
	It("should reject", func() {
		cfg := &v1alpha1.KubeletConfiguration{
			AutomaticReservations: &v1alpha1.AutomaticReservations{
				Profile: v1alpha1.ReservationProfileCapacityTiered,
			},
			KubeReserved: map[string]resource.Quantity{
				"cpu": resource.MustParse("500m"),
			},
		}
		cluster := createClusterWithKubeletConfig(cfg)
		req := createKubeletAdmissionRequest(cluster)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		validator = NewKubeletConfigurationValidator(client, decoder)

		resp := validator.validate(context.Background(), req)
		Expect(resp.Allowed).To(BeFalse())
		Expect(resp.Result.Message).To(ContainSubstring(
			"automaticReservations cannot be combined with",
		))
	})
})

Context("automaticReservations alone", func() {
	It("should accept", func() {
		cfg := &v1alpha1.KubeletConfiguration{
			AutomaticReservations: &v1alpha1.AutomaticReservations{
				Profile: v1alpha1.ReservationProfileCapacityTiered,
			},
		}
		cluster := createClusterWithKubeletConfig(cfg)
		req := createKubeletAdmissionRequest(cluster)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		validator = NewKubeletConfigurationValidator(client, decoder)

		resp := validator.validate(context.Background(), req)
		Expect(resp.Allowed).To(BeTrue())
	})
})
```

- [ ] **Step 2: Run to verify failure**

Run: `devbox run -- go test ./pkg/webhook/cluster/ -run TestCluster 2>&1 | tail -40`
(adjust to the package's Ginkgo suite entry func name if different)
Expected: FAIL — the reject case is allowed because no check exists yet.

- [ ] **Step 3: Add the check**

In `kubeletconfiguration_validator.go`, inside the `for _, entry := range cfgsToValidate` loop,
after the `entry.cfg == nil` continue and before the `cpuManagerPolicy` block, add:

```go
		if entry.cfg.AutomaticReservations != nil {
			if len(entry.cfg.SystemReserved) > 0 ||
				len(entry.cfg.KubeReserved) > 0 ||
				len(entry.cfg.EvictionHard) > 0 {
				return admission.Denied(fmt.Sprintf(
					"%s: automaticReservations cannot be combined with "+
						"systemReserved, kubeReserved, or evictionHard",
					entry.path,
				))
			}
		}
```

- [ ] **Step 4: Run to verify pass**

Run: `devbox run -- go test ./pkg/webhook/cluster/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/webhook/cluster/kubeletconfiguration_validator.go \
        pkg/webhook/cluster/kubeletconfiguration_validator_test.go
git commit -m "feat: [NCN-115115] Reject automaticReservations with explicit reservations"
```

### Task 6: Documentation

**Files:**

- Modify: `docs/content/customization/kubeadm/kubelet-configuration.md`

- [ ] **Step 1: Add a section** (after "Supported options", before "Default seccomp profile"):

````markdown
## Automatic resource reservations

Instead of hand-picking `systemReserved`/`kubeReserved` per node size, you can opt in to
automatic, node-size-aware reservations. Each node computes its `kubeReserved` (CPU and memory)
and a hard eviction threshold at boot, scaled to the node's actual capacity — the same approach
GKE and EKS use.

`automaticReservations` is mutually exclusive with `systemReserved`, `kubeReserved`, and
`evictionHard`; setting it alongside any of them is rejected at admission. Other kubelet fields
(such as `maxPods`) can still be set.

The `CapacityTiered` profile reserves:

- CPU: 6% of the first core, 1% of the second, 0.5% of cores three and four, 0.25% of each core
  beyond four.
- Memory: 255Mi below 1Gi total; otherwise 25% of the first 4Gi, 20% of the next 4Gi, 10% of the
  next 8Gi, 6% of the next 112Gi, and 2% of memory above 128Gi.
- A hard eviction threshold of `memory.available: 100Mi`.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    workers:
      machineDeployments:
      - class: default-worker
        name: md-0
        variables:
          overrides:
          - name: workerConfig
            value:
              kubeletConfiguration:
                automaticReservations:
                  profile: CapacityTiered
```
````

Add `automaticReservations` to the "Supported options" bullet list as well.

- [ ] **Step 2: Lint the docs**

Run: `devbox run -- markdownlint docs/content/customization/kubeadm/kubelet-configuration.md`
Expected: no findings (ATX headings, blank lines around headings/lists/fences, trailing newline).

- [ ] **Step 3: Commit**

```bash
git add docs/content/customization/kubeadm/kubelet-configuration.md
git commit -m "docs: [NCN-115115] Document automatic kubelet reservations"
```

### Task 7: Verify no handler version bump is required

- [ ] **Step 1: Confirm no patch-output change for inputs without the field**

The kubelet-configuration handler only emits the script/command when
`automaticReservations` is set, and `hasRenderableKubeletFields` preserves the exact previous
patch file for all other inputs. Verify by running the full package suite (which includes the
pre-existing render/no-op cases unchanged):

Run: `devbox run -- go test ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/...`
Expected: PASS with no edits required to any pre-existing expected-output assertion.

- [ ] **Step 2: Confirm default ClusterClasses are unchanged**

Run: `git diff --stat origin/main -- charts/cluster-api-runtime-extensions-nutanix/defaultclusterclasses`
Expected: empty (no default ClusterClass references the new field, so no rollout on upgrade).

No version bump per `.cursor/rules/handler-version-safety.mdc` (opt-in, output unchanged for
existing inputs). No commit for this task.

### Task 8: Full lint + test pass

- [ ] **Step 1: Lint**

Run: `devbox run -- golangci-lint run ./api/... ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/... ./pkg/webhook/cluster/...`
Expected: no findings.

- [ ] **Step 2: Test the affected packages**

Run:

```bash
devbox run -- gotestsum --format pkgname -- \
  ./api/v1alpha1/... \
  ./pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/... \
  ./pkg/webhook/cluster/...
```

Expected: all pass.

- [ ] **Step 3: Verify pre-commit generated artifacts are in sync**

Run: `devbox run -- make generate && git diff --exit-code`
Expected: exit 0 (no uncommitted generated drift).

### Task 9 (optional): Manual smoke verification

- [ ] Build the operator image and deploy to a kind-based management cluster; create a workload
  cluster with two worker MachineDeployments of different sizes and
  `workerConfig.kubeletConfiguration.automaticReservations.profile: CapacityTiered`.
- [ ] On each worker node, confirm `/etc/kubernetes/patches/kubeletconfiguration50+strategic.json`
  exists with size-appropriate `kubeReserved`, and that
  `kubectl get node <n> -o jsonpath='{.status.capacity.memory} {.status.allocatable.memory}'`
  shows allocatable reduced by the reserved amount, with the larger node reserving a smaller
  fraction.

### Task 10: End-to-end test (all providers, no rollout)

**Files:**

- Create: `test/e2e/kubelet_reservations_test.go`
- Create: `test/e2e/kubelet_reservations_helpers.go`
- Create: `test/e2e/kubelet_reservations_helpers_test.go`
- Modify: `test/e2e/quick_start_test.go` (extract shared setup helpers)

**Approach (no MachineDeployment rollout):** Patching `workerConfig` on a live cluster mutates the
`KubeadmConfigTemplate` and forces a worker rollout. Instead, the test generates a dedicated flavor
at runtime by reading the published quick-start example
(`cluster-template-topology-cilium-helm-addon.yaml`), enabling
`workerConfig.kubeletConfiguration.automaticReservations.profile: CapacityTiered` on it (merged
into existing provider machine details, and into any per-MachineDeployment override), writing the
patched template to a temp dir, and registering it as a new flavor in a deep-copied `E2EConfig`.
The worker nodes therefore come up already configured — the reservation is applied on first boot
with no rollout. The published examples are never modified. One combination (Cilium + HelmAddon)
per provider exercises the boot-time mechanism end to end.

- `enableAutomaticReservationsInClusterTemplate` does the YAML merge (unit-tested in
  `kubelet_reservations_helpers_test.go`: preserves provider machine details, leaves `clusterConfig`
  untouched, patches per-MD overrides, survives `${...}` envsubst placeholders, rejects non-Cluster
  docs).
- `registerAutomaticReservationsFlavor` resolves the base flavor's (now-absolute) `SourcePath` from
  the deep-copied config, writes the patched temp template, and appends a `cluster-template-…-kubelet-reservations.yaml`
  flavor entry.
- `assertWorkerNodesHaveReservedResources` asserts every worker node reports allocatable CPU and
  memory strictly below capacity. CPU is the decisive signal: default kubeadm reserves no CPU, so
  allocatable CPU below capacity can only come from the injected `kubeReserved`.
- Shared setup (`applyProviderKubernetesVersionOverride`, `reserveNutanixIPsForCluster`) is extracted
  from `quick_start_test.go` and reused, avoiding duplication.

**Rootless Docker (opt-in, default rootful):** Under rootless Docker the CAPD workload kubelet
crash-loops with `failed to create kubelet: open /dev/kmsg: operation not permitted`, so the
control plane never initializes and the worker assertion is never reached. Setting
`CAREN_E2E_KUBELET_IN_USERNS=true` makes `maybeEnableKubeletInUserNamespace` patch the quick-start
ClusterClass at runtime — injecting `feature-gates: KubeletInUserNamespace=true` into every
`kubeletExtraArgs` list (CP init/join and worker join) of the `KubeadmControlPlaneTemplate` and
`KubeadmConfigTemplate` — written to a temp copy with the `SourcePath` repointed before the
clusterctl repository is built. The published ClusterClass is never modified, and the default
(env unset) rootful CI path is unchanged. Unit-tested in `kubelet_reservations_helpers_test.go`
(`TestEnableKubeletInUserNamespaceInClusterClass`, `TestEnableKubeletInUserNamespaceExtendsExistingFeatureGates`).
Pair with the configurable bootstrap Docker socket (`CAREN_E2E_DOCKER_SOCKET` / `DOCKER_HOST`).

- [ ] **Verify:** `devbox run -- go vet -tags e2e ./test/e2e/...`,
  `devbox run -- go test -tags e2e ./test/e2e/ -run TestEnableAutomaticReservations`, and
  `devbox run -- ./hack/tools/golangci-lint-kube-api-linter run --config=.golangci.yml ./test/e2e/...`
  all pass. Full e2e (`make e2e-test`) runs the new spec per provider via the `kubelet-reservations`
  label.

## Self-Review

**Spec coverage:**

- FR-001 (opt-in field, profile, both scopes) → Task 1 (field + enum) + Task 3 (worker + CP).
- FR-002 (boot-time, before kubeadm, patches dir) → Task 2 (script) + Task 3 (preKubeadmCommand).
- FR-003 (CPU formula) → Task 2 script + test cases.
- FR-004 (memory formula) → Task 2 script + test cases.
- FR-005 (kubeReserved + evictionHard 100Mi) → Task 2 script + test assertions.
- FR-006 (mutual exclusivity, declarative) → Task 1 CEL marker + Task 5 webhook.
- FR-007 (other kubelet fields honoured) → Task 3 (`maxPods` + auto path; `hasRenderableKubeletFields`).
- FR-008 (no-op when unset, no version bump) → Task 4 + Task 7.
- FR-009 (deterministic testability via overrides) → Task 2 env overrides + test.
- FR-010 (docs) → Task 6.

**Placeholder scan:** none — every code/script/test step contains complete content.

**Type consistency:** `AutomaticReservations`, `ReservationProfile`,
`ReservationProfileCapacityTiered`, `automaticReservationsEnabled`,
`computeReservationsScript`, `computeReservationsScriptFile`, `computeReservationsScriptPath`,
`computeReservationsCommand`, `hasRenderableKubeletFields` are defined once (Tasks 1-3) and used
consistently in later tasks. Patch filename `kubeletconfiguration50+strategic.json` matches
between the script (Task 2) and the script test (Task 2). Memory worked examples (1843Mi/2662Mi/
5611Mi/17407Mi) are consistent between the spec and the Task 2 test table.
