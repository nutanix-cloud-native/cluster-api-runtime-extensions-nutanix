# Why New Machines Are Rolled Out Despite In-Place Update Extension

When using ClusterClass/topology with the in-place update extension, bootstrap config changes (e.g. adding image registry credentials, new files) can still trigger a **full MachineSet rollout** instead of in-place updates. This note explains why, based on CAPI controller logs in this folder.

## Root Cause

The **topology controller** (`reconcile_state.go`) rotates the bootstrap template **before** the in-place update extension is consulted. Once it rotates, the MachineDeployment's `spec.template.spec.bootstrap.configRef.name` points to a **new** KubeadmConfigTemplate. The existing MachineSet still references the **old** template name, so the MachineDeployment controller finds no matching MachineSet and creates a new one, triggering a rollout.

## Order of Operations (from logs)

1. **Topology controller** sees desired KubeadmConfigTemplate content ≠ current (e.g. new files, registry config).
2. Topology **rotates** the template: creates a **new** KubeadmConfigTemplate with a new hash-based name (e.g. `...-zzgvx`) and patches the MachineDeployment to reference it (configRef: `6nn2s` → `zzgvx`).
3. **MachineDeployment controller** runs. It sees that no MachineSet matches the MachineDeployment's template (existing MachineSet has configRef `6nn2s`, template now requires `zzgvx`).
4. MachineDeployment controller logs: *"couldn't find MachineSet matching MachineDeployment spec template: ... spec.bootstrap.configRef KubeadmConfigTemplate dk-in-place-update-demo-md-0-6nn2s, KubeadmConfigTemplate dk-in-place-update-demo-md-0-zzgvx required"*.
5. It creates a **new** MachineSet and scales it up (rollout).
6. The in-place update extension is consulted **after** rotation: *"MachineSet ... can be updated in-place by extensions"* — but by then the ref has already changed, so the existing MachineSet is no longer the "matching" one.

## Summary Table

| Component | Behavior |
|-----------|----------|
| **Topology controller** (`reconcile_state.go`) | On bootstrap template content change, it **always rotates** (new template name + patch MachineDeployment). It does **not** consult the in-place update extension before rotating. |
| **MachineDeployment controller** | After the ref is updated, no MachineSet matches the new template → creates new MachineSet and rolls out. The extension's "can update in place" applies to the *old* MachineSet; the MD's *template* already points to the new template name. |

## CAPI Version Note

Tested with CAPI v1.12.2. The v1.12.1 fix (#13147, "Preserve existing object names for backward compatibility with pre-v1.7 in-place updates") applies to the MachineDeployment/MachineSet in-place update path when **applying** patches, not to the **topology** controller's decision to rotate. So rotation still happens when topology detects a template content diff.

## What Would Need to Change (in CAPI)

For true in-place updates when only bootstrap *content* changes (same template object), the **topology** controller would need to:

- Consult the in-place update extension (e.g. `CanUpdateMachineSet`) **before** deciding to rotate, and
- If the extension says it can update in place and returns a patch, **patch the existing KubeadmConfigTemplate** and **leave** `configRef.name` unchanged instead of creating a new template and updating the ref.

## References

- Log excerpts: `logs.log`, `full-log.log` in this directory (see "Rotating KubeadmConfigTemplate", "Patching MachineDeployment", "couldn't find MachineSet matching MachineDeployment spec template").
- CAPI topology: `reconcile_state.go` (template rotation and MachineDeployment patch).
- CAPI MachineDeployment: `machinedeployment_controller.go`, `machinedeployment_canupdatemachineset.go`.