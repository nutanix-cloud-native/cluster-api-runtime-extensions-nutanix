<!--
 Copyright 2023 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Run the CAREN checks on a dev VM (pre-PR gate)

Run the same checks CI runs — on a Prism Central VM created on demand —
against your **local working tree**, before you commit or push.

You never log into the VM. One command from your laptop provisions it, runs the
checks, copies the results back, and deletes the VM.

---

## 1. One-time setup (~2 minutes)

```bash
gh auth login --web     # avoids anonymous GitHub API rate limits during tool downloads
```

You also need to be **on VPN** (the PC subnet must be reachable) and have
`python3`, `ssh` and `git` — all standard on macOS.

CAREN has no private Go module dependencies, so no SAML SSO gymnastics are
needed (unlike konvoy2 or kommander).

## 2. Get Prism Central credentials

They rotate every few days, so fetch fresh ones when a run fails with `401`:

```bash
cd <your tam-cli checkout> && ./tam login pc-dev
```

It prints ready-to-paste `export NUTANIX_USER=… / export NUTANIX_PASSWORD=…`
lines.

## 3. Run the checks

```bash
export NUTANIX_USER=<pc-username>
export NUTANIX_PASSWORD=<pc-password>

cd <your CAREN checkout>
./hack/dev-vm/caren-pc-test.sh run
```

Or through the pre-commit hook that ships with this repo:

```bash
pre-commit run caren-pc-test --hook-stage manual
```

### What exactly gets tested

Your **local working tree** — committed, uncommitted and untracked files alike.
No commit or push required; that is the point of a pre-commit gate.

Mechanically the VM clones the repo history from `origin` (fast, and needed for
tags), and the script ships only your delta on top of it: commits you have not
pushed as a git bundle, edits to tracked files as a patch, and untracked files
as a tarball. Files ignored by `.gitignore` are not sent. `RESULTS.md` records
when a run came from a dirty tree.

### Suites

The default set is **every blocking job** in `.github/workflows/checks.yml`, so
a green run here means CI should be green too (~40 min). `helm` costs about a
minute unless you touched a chart — it runs the same `ct list-changed` check CI
does — and `e2e-docker` runs one focus rather than CI's full matrix.

```bash
./hack/dev-vm/caren-pc-test.sh run --suites "precommit unit lint"  # fast loop, ~25 min
E2E_FOCUS='Self-hosted' ./hack/dev-vm/caren-pc-test.sh run --suites "e2e-docker"
CT_FORCE=true ./hack/dev-vm/caren-pc-test.sh run --suites "helm"   # force the chart install
./hack/dev-vm/caren-pc-test.sh run --keep                          # keep the VM afterwards
./hack/dev-vm/caren-pc-test.sh list                                # your test VMs on PC
./hack/dev-vm/caren-pc-test.sh destroy <vm-uuid>                   # manual cleanup
```

| Suite | Mirrors CI job | What it runs | Typical duration |
|---|---|---|---|
| `precommit` | `pre-commit` | `make pre-commit` with CI's `SKIP` list | ~5 min |
| `unit` | `unit-test` | `make test` — root, `api` and `common` modules | ~15 min |
| `lint` | `lint-go` | `make lint` — custom `golangci-lint-kube-api-linter`, all modules | ~5 min |
| `helm` | `lint-test-helm` | `ct lint`, kind cluster, `release-snapshot`, `clusterctl.init`, `ct install` | ~1 min, ~15 min if charts changed |
| `e2e-docker` | `e2e-quick-start` / `e2e-self-hosted` | `make e2e-test E2E_LABEL='provider:Docker'` | ~13 min per focus |

The Nutanix-provider e2e suites are not wired up here: they need a full CAPX
environment (PC endpoint, subnet, machine image) beyond the credentials this
script uses to create the VM.

## 4. Results

Everything lands in `test-results/<timestamp>/` on your laptop:

```text
test-results/20260803-102619/
├── RESULTS.md          # pass/fail table + branch, commit sha, runner — attach to your PR
├── results.txt         # raw per-suite verdicts
└── artifacts/
    ├── unit/           # suite log, test.json, junit-report.xml
    ├── lint/           # suite log
    └── …               # one folder per suite
```

Attach `RESULTS.md` to the PR when you raise it — paste it (it renders as a
table) or screenshot it.

## Configuration reference

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `NUTANIX_USER` | **yes** | — | Prism Central username (VM create/delete) |
| `NUTANIX_PASSWORD` | **yes** | — | Prism Central password (never logged) |
| `VM_CPU` / `VM_MEM_GIB` / `VM_DISK_GIB` | no | `16` / `32` / `200` | VM sizing |
| `CAREN_TEST_SUITES` | no | all five suites | Change your default suite list |
| `E2E_FOCUS` | no | `Quick start` | Which e2e focus `e2e-docker` runs |
| `CT_FORCE` | no | `false` | Run the chart install even when no charts changed |
| `CAREN_REPO_URL` | no | the public repo | Point at the `internal-` fork if that is what you work in |
| `PC_URL` | no | `https://pc.dev.nkp.sh:9440` | Prism Central endpoint |
| `IMAGE_UUID` | no | Ubuntu 24.04 cloud image | Boot image (cloud-init-enabled DISK_IMAGE) |
| `SUBNET_UUID` | no | `vlan170-dhcp` | NIC subnet (DHCP) |
| `PE_CLUSTER_UUID` | no | `ncn-dev-sandbox` | PE cluster to place the VM on |

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `Set NUTANIX_USER and NUTANIX_PASSWORD` | the two env vars aren't exported in this shell |
| `HTTP 401 … Invalid Credentials` | PC credentials rotated — re-run `tam login pc-dev` |
| Hangs at *waiting for IP* / SSH | VPN dropped, or the PE is out of capacity — retry with smaller `VM_CPU`/`VM_MEM_GIB` |
| A suite fails | the VM is **kept**; the script prints the `ssh` command to debug it and the `destroy` command to clean up |
| Docker Hub rate limits (429) | `docker login` on the VM with your own account, then re-run the suite |

## Advanced: a long-lived DevVM

`caren-test-vm.sh` is the VM-side half and works standalone if you keep a
personal DevVM instead of creating one per run:

```bash
scp hack/dev-vm/caren-test-vm.sh <user>@<vm-ip>:
ssh <user>@<vm-ip>
./caren-test-vm.sh setup          # one-time: apt, docker, nix, devbox, gh
# log out and back in (docker group + nix PATH)
./caren-test-vm.sh clone my-branch
./caren-test-vm.sh pre-pr         # precommit + unit + lint
```
