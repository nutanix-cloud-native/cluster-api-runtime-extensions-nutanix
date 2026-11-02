<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# `v6` drift report: `v0.50.0` against `main`

**Jira Ticket**: [NCN-107121](https://jira.nutanix.com/browse/NCN-107121)
**Compared**: `v0.50.0` (`9bc64546`) against `main` at `aac327fc`
**Result**: no drift. `v6` on `main` produces the same patches as `v6` in `v0.50.0` for every corpus input.

## Method

1. `git worktree add --detach .worktrees/baseline-v0.50.0 v0.50.0`.
2. Copied `pkg/handlers/golden/` (harness, tests, all 27 corpus inputs) and the `Mutators()` accessor in
   `common/pkg/capi/clustertopology/handlers/mutation/meta.go` into it. Golden files were not copied.
3. Adapted the copy to CAPI v1.12.5. These changes only affect how the harness builds requests, not handler code:
   - builtin `metadata` is `clusterv1beta1.ObjectMeta` (same JSON as v1.13's `runtimehooksv1.ObjectMeta`);
   - discovered variables are `v1beta1`, converted with `Convert_v1beta1_ClusterClassVariable_To_v1beta2_ClusterClassVariable`
     as CAPI v1.12's ClusterClass controller does;
   - `ptr.To` instead of Go 1.26 `new(expr)`.
4. Ran `TestGolden` with `-update-golden=v6` and copied the `v6` and discovery golden files to
   `/tmp/caren-baseline-v0.50.0/`. No input produced a validation error or a handler error.
5. Diffed against `pkg/handlers/golden/testdata/golden` on `NCN-107121-01-golden-harness`.

## Results

| Golden files | Count | Byte-identical |
| --- | --- | --- |
| `v6` patch output, all providers, cluster and worker | 108 | 108 |
| Discovery (`*ConfigVars/variables.yaml`) | 9 | 0 |

The discovery files differ only in serialization. CAPI v1.13 changed the `DiscoverVariablesResponse` schema fields to
`*bool`, so `main` writes explicit `false` values (`exclusiveMaximum`, `uniqueItems`,
`x-kubernetes-preserve-unknown-fields`, ...) and omits the empty `metadata: {}`. After dropping `false` and `{}` values
on both sides, all 9 files are identical. CAPI stores and compares the schema semantically, so this difference does
not change Cluster variables or roll Machines.

## After rebasing onto `257745de`

`main` moved to CAPI v1.14.3 (#1729). All `v6` and `v7` patch golden files are unchanged. The same PR re-synced the
CAPX API, which adds an optional GPU `profile` field (and a `profile` value for `type`) to the unversioned
`NutanixClusterConfigVars` and `NutanixWorkerConfigVars` schemas. The change is additive and has no defaults, so
existing Cluster variables stay valid and unchanged and nothing rolls. It is still a change to a released schema: PR 5
pins `v6` discovery to the `v0.50.0` schema, without this field.

## Consequences for the plan

- PR 6 (align `v6` with `v0.50.0`) is not needed; drop layer `NCN-107121-05-align-v6`.
- PR 5 must not replace the discovery golden files with the bytes of this baseline, because those were serialized with
  CAPI v1.12 types. Compare after the normalization above instead, or regenerate them on `main` from the frozen
  `v6` schema.
- The baseline is kept in `.worktrees/baseline-v0.50.0/pkg/handlers/golden/testdata/golden` for re-checks after
  rebasing the stack (see "Local-first delivery" in `plan.md`).
