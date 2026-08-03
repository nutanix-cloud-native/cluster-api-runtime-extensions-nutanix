#!/usr/bin/env bash
# shellcheck disable=SC1090,SC1091,SC2016,SC2029
#
# caren-pc-test.sh — run CAREN checks on a dynamic VM on Prism Central.
#
# Developer flow (from your laptop, inside the CAREN repo):
#   export NUTANIX_USER=<pc-username>
#   export NUTANIX_PASSWORD=<pc-password>
#   ./hack/dev-vm/caren-pc-test.sh run                 # full pre-PR gate
#   ./hack/dev-vm/caren-pc-test.sh run --suites "unit lint"   # subset
#   ./hack/dev-vm/caren-pc-test.sh list                # your test VMs on PC
#   ./hack/dev-vm/caren-pc-test.sh destroy <vm-uuid>   # manual cleanup
#
# What `run` does:
#   1. provisions an Ubuntu 24.04 VM on PC (16 vCPU / 64 GiB / 200 GiB) with an
#      ephemeral SSH key via cloud-init
#   2. bootstraps it exactly like the GitHub CI runners (caren-test-vm.sh setup)
#   3. transfers your `gh` token (never printed) for private-repo access
#   4. clones your CURRENT BRANCH (must be pushed) and runs the suites
#   5. pulls back test-results/<ts>/RESULTS.md + logs; deletes the VM on success
#      (kept for debugging on failure — destroy with the printed command)
#
# Requirements: bash, python3, ssh, git, gh (authenticated, SSO for
# mesosphere + nutanix-cloud-native), network access to the PC subnet (VPN).
#
# Timing: full suite set is ~2.5-3h. Use as a pre-PR gate, e.g.:
#   pre-commit run caren-pc-test --hook-stage manual
# Not suitable as a per-commit hook.

set -euo pipefail

# --- Configuration (defaults = Nutanix dev PC; override via env) ---------------
PC_URL="${PC_URL:-https://pc.dev.nkp.sh:9440}"
# rohith-24.04-ubuntu-server-cloudimg-amd64.img (Ubuntu 24.04 cloud image)
IMAGE_UUID="${IMAGE_UUID:-ce5c64be-7896-4fd9-9695-ad8b63f0eda8}"
# subnet vlan170-dhcp
SUBNET_UUID="${SUBNET_UUID:-cd32cd27-ffd9-4bde-88e5-cb5167d7bd17}"
# PE cluster ncn-dev-sandbox
PE_CLUSTER_UUID="${PE_CLUSTER_UUID:-00061f7f-44f7-19dc-3be1-7cc25586ee44}"
VM_CPU="${VM_CPU:-16}"
VM_MEM_GIB="${VM_MEM_GIB:-32}"
VM_DISK_GIB="${VM_DISK_GIB:-200}"
VM_USER="nkp"
# The blocking jobs from .github/workflows/checks.yml. The heavier `helm` and
# `e2e-docker` suites are available but opt-in (see --suites).
DEFAULT_SUITES="${CAREN_TEST_SUITES:-precommit unit lint}"

STATE_DIR="$HOME/.caren-pc-test"
SSH_KEY="$STATE_DIR/id_ed25519"
SSH_OPTS=(-i "$SSH_KEY" -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10)

log() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
die() {
  printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2
  exit 1
}

require_creds() {
  if [ -z "${NUTANIX_USER:-}" ] || [ -z "${NUTANIX_PASSWORD:-}" ]; then
    die "Set NUTANIX_USER and NUTANIX_PASSWORD env vars (your PC credentials)"
  fi
}

# --- PC API helper: creds from env (never argv/logs), body from stdin ----------
pc_api() { # pc_api <method> <path> [json-body-on-stdin]
  local body=""
  [ -t 0 ] || body=$(cat)
  PC_BODY="$body" python3 -c '
import base64, os, ssl, sys, urllib.request, urllib.error
method, path = sys.argv[1], sys.argv[2]
body = os.environ.get("PC_BODY", "")
data = body.encode() if body.strip() else None
ctx = ssl.create_default_context(); ctx.check_hostname = False; ctx.verify_mode = ssl.CERT_NONE
u = os.environ["NUTANIX_USER"]; p = os.environ["NUTANIX_PASSWORD"]
auth = base64.b64encode((u + ":" + p).encode()).decode()
req = urllib.request.Request(os.environ["PC_URL"] + path, method=method, data=data)
req.add_header("Authorization", "Basic " + auth)
req.add_header("Content-Type", "application/json")
try:
    with urllib.request.urlopen(req, context=ctx) as r:
        sys.stdout.write(r.read().decode())
except urllib.error.HTTPError as e:
    sys.stderr.write(f"HTTP {e.code}: {e.read().decode()[:500]}\n"); sys.exit(1)
' "$1" "$2"
}
export PC_URL

# --- Provisioning ---------------------------------------------------------------
ensure_ssh_key() {
  mkdir -p "$STATE_DIR"
  [ -f "$SSH_KEY" ] || ssh-keygen -t ed25519 -N "" -f "$SSH_KEY" -C "caren-pc-test" >/dev/null
}

vm_name() { echo "caren-test-$(whoami | tr -cd 'a-z0-9')-$(date +%y%m%d%H%M%S)"; }

provision_vm() { # -> sets VM_UUID, VM_IP
  local name
  name=$(vm_name)
  local pubkey
  pubkey=$(cat "$SSH_KEY.pub")
  log "Creating VM $name on PC ($VM_CPU vCPU / ${VM_MEM_GIB}G / ${VM_DISK_GIB}G)" >&2
  local user_data
  user_data=$(
    cat <<EOF
#cloud-config
hostname: $name
users:
  - name: $VM_USER
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: sudo
    shell: /bin/bash
    ssh-authorized-keys:
      - $pubkey
growpart:
  mode: auto
  devices: ["/"]
EOF
  )
  local b64
  b64=$(printf '%s' "$user_data" | base64 | tr -d '\n')
  local resp
  resp=$(
    pc_api POST /api/nutanix/v3/vms <<EOF
{"api_version":"3.1","metadata":{"kind":"vm"},
 "spec":{"name":"$name",
  "cluster_reference":{"kind":"cluster","uuid":"$PE_CLUSTER_UUID"},
  "resources":{
   "num_sockets":2,"num_vcpus_per_socket":$((VM_CPU / 2)),"memory_size_mib":$((VM_MEM_GIB * 1024)),
   "power_state":"ON",
   "boot_config":{"boot_device_order_list":["DISK","CDROM","NETWORK"],"boot_type":"UEFI"},
   "disk_list":[{"device_properties":{"device_type":"DISK","disk_address":{"adapter_type":"SCSI","device_index":0}},
                 "data_source_reference":{"kind":"image","uuid":"$IMAGE_UUID"},
                 "disk_size_mib":$((VM_DISK_GIB * 1024))}],
   "nic_list":[{"nic_type":"NORMAL_NIC","subnet_reference":{"kind":"subnet","uuid":"$SUBNET_UUID"}}],
   "guest_customization":{"cloud_init":{"user_data":"$b64"}}}}}
EOF
  )
  VM_UUID=$(printf '%s' "$resp" | python3 -c 'import json,sys; print(json.load(sys.stdin)["metadata"]["uuid"])')
  echo "$VM_UUID" >"$STATE_DIR/last-vm-uuid"
  log "VM UUID: $VM_UUID - waiting for IP (DHCP + boot, ~2-4 min)" >&2
  local ip=""
  for _ in $(seq 1 60); do
    ip=$(pc_api GET "/api/nutanix/v3/vms/$VM_UUID" </dev/null | python3 -c '
import json,sys
d=json.load(sys.stdin)
ips=[e.get("ip") for n in d.get("status",{}).get("resources",{}).get("nic_list",[]) for e in n.get("ip_endpoint_list",[]) if e.get("ip")]
print(ips[0] if ips else "")')
    [ -n "$ip" ] && break
    sleep 10
  done
  [ -n "$ip" ] || die "VM got no IP after 10 min - check PC console (uuid $VM_UUID)"
  VM_IP="$ip"
  log "VM IP: $VM_IP - waiting for SSH" >&2
  for _ in $(seq 1 40); do
    ssh "${SSH_OPTS[@]}" "$VM_USER@$VM_IP" true 2>/dev/null && return 0
    sleep 10
  done
  die "SSH to $VM_IP not reachable after 7 min"
}

destroy_vm() { # <uuid>
  log "Deleting VM $1"
  pc_api DELETE "/api/nutanix/v3/vms/$1" </dev/null >/dev/null
  echo "deleted"
}

# --- Test run -------------------------------------------------------------------
KEEP=false
cmd_run() {
  local suites="$DEFAULT_SUITES"
  while [ $# -gt 0 ]; do
    case "$1" in
    --suites)
      suites="$2"
      shift 2
      ;;
    --keep)
      KEEP=true
      shift
      ;;
    *) die "unknown flag $1" ;;
    esac
  done

  require_creds
  command -v gh >/dev/null || die "gh CLI required on your laptop"
  gh auth status >/dev/null 2>&1 || die "Run: gh auth login (account with mesosphere + nutanix-cloud-native access)"
  local repo_root
  repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || die "Run from inside the CAREN repo"
  local branch sha
  branch=$(git -C "$repo_root" branch --show-current)
  sha=$(git -C "$repo_root" rev-parse --short HEAD)
  [ -n "$branch" ] || die "Detached HEAD - checkout a branch"

  # Snapshot the LOCAL working tree so you can gate changes before committing or
  # pushing. The VM clones the repo history from origin (fast), and we ship only
  # the delta: commits you have not pushed (as a git bundle), tracked-file edits
  # (as a patch) and untracked files (as a tarball). That keeps the transfer in
  # the KB range instead of rsyncing a multi-GB working tree over the VPN.
  local snap
  snap=$(mktemp -d)
  # shellcheck disable=SC2064
  trap "rm -rf '$snap'" RETURN
  # Always produce all three files (empty is fine) so the transfer below is a
  # single all-or-nothing step: a missing file would make scp fail as a whole
  # and silently skip the others.
  git -C "$repo_root" bundle create "$snap/local.bundle" HEAD --not --remotes=origin \
    >/dev/null 2>&1 || : # fails (writes nothing) when HEAD is already on origin
  [ -f "$snap/local.bundle" ] || : >"$snap/local.bundle"
  git -C "$repo_root" diff HEAD --binary >"$snap/uncommitted.patch"
  # ':(exclude)test-results' keeps this script's own output out of the payload
  # even on branches that predate the .gitignore entry for it.
  (cd "$repo_root" &&
    git ls-files --others --exclude-standard -z -- ':(exclude)test-results' \
      >"$snap/untracked.list")
  if [ -s "$snap/untracked.list" ]; then
    (cd "$repo_root" && tar --null -czf "$snap/untracked.tgz" -T "$snap/untracked.list")
  else
    : >"$snap/untracked.tgz"
  fi

  local dirty=false
  if [ -s "$snap/uncommitted.patch" ] || [ -s "$snap/local.bundle" ] ||
    [ -n "$(cd "$repo_root" && git ls-files --others --exclude-standard | head -1)" ]; then
    dirty=true
    log "Testing your LOCAL working tree (uncommitted and/or unpushed changes included)"
  else
    log "Working tree matches origin/$branch - testing $sha"
  fi

  ensure_ssh_key
  provision_vm # sets VM_UUID VM_IP
  trap '[ "${KEEP:-false}" = true ] || { echo "Cleaning up VM..."; destroy_vm "$VM_UUID" || true; }' EXIT

  log "Bootstrapping VM (CI-parity toolchain: docker, nix, devbox, gh - ~10 min)"
  ssh "${SSH_OPTS[@]}" "$VM_USER@$VM_IP" 'mkdir -p ~/bin'
  scp -q -i "$SSH_KEY" -o BatchMode=yes -o StrictHostKeyChecking=accept-new \
    "$repo_root/hack/dev-vm/caren-test-vm.sh" "$VM_USER@$VM_IP:caren-test-vm.sh"
  ssh "${SSH_OPTS[@]}" "$VM_USER@$VM_IP" 'chmod +x caren-test-vm.sh && ./caren-test-vm.sh setup' |
    grep -E "==>|ERROR" || true

  log "Transferring GitHub token (not printed) + your local working tree"
  gh auth token | ssh "${SSH_OPTS[@]}" "$VM_USER@$VM_IP" 'gh auth login --with-token && gh auth setup-git'
  ssh "${SSH_OPTS[@]}" "$VM_USER@$VM_IP" 'rm -rf ~/localsync && mkdir -p ~/localsync'
  scp -q -i "$SSH_KEY" -o BatchMode=yes "$snap/local.bundle" "$snap/uncommitted.patch" \
    "$snap/untracked.tgz" "$VM_USER@$VM_IP:localsync/" ||
    die "failed to transfer the local working-tree snapshot to the VM"
  # sg docker: group membership from setup isn't active in this ssh session yet
  ssh "${SSH_OPTS[@]}" "$VM_USER@$VM_IP" "sg docker -c './caren-test-vm.sh sync-local $sha'" |
    tail -3

  log "Running suites: $suites"
  local ts results_dir
  ts=$(date +%Y%m%d-%H%M%S)
  results_dir="$repo_root/test-results/$ts"
  mkdir -p "$results_dir"
  ssh "${SSH_OPTS[@]}" "$VM_USER@$VM_IP" "cat > ~/run-suites.sh <<'EOF'
#!/usr/bin/env bash
set -u
: > ~/results.txt
rm -rf ~/artifacts && mkdir -p ~/artifacts
for s in $suites; do
  echo \"=== \$s start \$(date -u +%H:%M:%S) ===\"
  start=\$SECONDS
  if sg docker -c \"./caren-test-vm.sh \$s\" > ~/suite-\$s.log 2>&1; then
    echo \"\$s PASS \$((SECONDS-start))s\" >> ~/results.txt
  else
    echo \"\$s FAIL \$((SECONDS-start))s\" >> ~/results.txt
  fi
  # Preserve this suite's artifacts before the next one overwrites them:
  # go test json output, ginkgo/junit reports and the full log.
  mkdir -p ~/artifacts/\$s
  cp ~/suite-\$s.log ~/artifacts/\$s/
  find ~/caren -maxdepth 2 \( -name 'test.json' -o -name 'junit*.xml' \) -exec mv {} ~/artifacts/\$s/ \; 2>/dev/null
  find ~/caren/_artifacts -maxdepth 2 -name '*.xml' -exec cp {} ~/artifacts/\$s/ \; 2>/dev/null || true
  sg docker -c 'for c in \$(kind get clusters 2>/dev/null); do kind delete cluster --name \"\$c\"; done; docker system prune -f' >/dev/null 2>&1
  tail -1 ~/results.txt
done
EOF
chmod +x ~/run-suites.sh && ~/run-suites.sh"

  log "Collecting results"
  scp -q -i "$SSH_KEY" -o BatchMode=yes "$VM_USER@$VM_IP:results.txt" "$results_dir/results.txt"
  ssh "${SSH_OPTS[@]}" "$VM_USER@$VM_IP" 'tar -C ~ -czf - artifacts 2>/dev/null' >"$results_dir/artifacts.tar.gz" || true
  tar -xzf "$results_dir/artifacts.tar.gz" -C "$results_dir" 2>/dev/null || true

  # RESULTS.md - the artifact developers attach to their PR
  local overall="PASSED"
  grep -q FAIL "$results_dir/results.txt" && overall="FAILED"
  {
    echo "## CAREN pre-PR test results: $overall"
    echo
    echo "- **Branch:** \`$branch\` @ \`$sha\`$([ "$dirty" = true ] && echo " (local working tree, including uncommitted/unpushed changes)")"
    echo "- **Date:** $(date -u '+%Y-%m-%d %H:%M UTC')"
    echo "- **Runner:** dynamic PC VM ($VM_CPU vCPU / ${VM_MEM_GIB}G, Ubuntu 24.04, CI-parity devbox toolchain)"
    echo
    echo "| Suite | Result | Duration |"
    echo "|---|---|---|"
    awk '{printf "| %s | %s | %s |\n", $1, ($2=="PASS"?"✅ PASS":"❌ FAIL"), $3}' "$results_dir/results.txt"
    echo
    echo "<details><summary>Artifacts collected per suite</summary>"
    echo
    (cd "$results_dir" && find artifacts -type f 2>/dev/null | sort | sed 's/^/- `/;s/$/`/')
    echo
    echo "</details>"
    echo
    echo "_Generated by hack/dev-vm/caren-pc-test.sh; full logs + JUnit XMLs + coverage in this directory._"
  } >"$results_dir/RESULTS.md"

  cat "$results_dir/RESULTS.md"
  log "Results in $results_dir (attach RESULTS.md when you raise the PR)"

  if [ "$overall" = "FAILED" ]; then
    KEEP=true
    echo "Suites failed - VM kept for debugging: ssh -i $SSH_KEY $VM_USER@$VM_IP"
    echo "Destroy later with: $0 destroy $VM_UUID"
    exit 1
  fi
}

cmd_list() {
  require_creds
  pc_api POST /api/nutanix/v3/vms/list <<<'{"kind":"vm","filter":"vm_name==caren-test-.*","length":100}' |
    python3 -c '
import json,sys
for e in json.load(sys.stdin).get("entities",[]):
    r=e["status"]["resources"]
    ips=[i.get("ip") for n in r.get("nic_list",[]) for i in n.get("ip_endpoint_list",[]) if i.get("ip")]
    print(e["metadata"]["uuid"], e["status"]["name"], r.get("power_state"), ",".join(ips))'
}

cmd_destroy() {
  require_creds
  [ -n "${1:-}" ] || die "usage: $0 destroy <vm-uuid>"
  destroy_vm "$1"
}

case "${1:-}" in
run)
  shift
  cmd_run "$@"
  ;;
list)
  shift
  cmd_list "$@"
  ;;
destroy)
  shift
  cmd_destroy "$@"
  ;;
*)
  sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
  exit 1
  ;;
esac
