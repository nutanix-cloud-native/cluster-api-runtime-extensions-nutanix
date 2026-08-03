#!/usr/bin/env bash
# shellcheck disable=SC1090,SC1091,SC2016,SC2029
#
# caren-test-vm.sh — bootstrap a VM and run the CAREN checks on it.
#
# Sets a fresh Ubuntu VM up like the GitHub Actions runner and runs the same
# targets .github/workflows/checks.yml runs, so a developer can gate a change
# before committing it.
#
# Usage:
#   ./caren-test-vm.sh setup                # one-time VM bootstrap (idempotent)
#   ./caren-test-vm.sh clone [branch]       # clone the repo (optional branch)
#   ./caren-test-vm.sh sync-local <sha>     # reproduce laptop's tree (used by caren-pc-test.sh)
#   ./caren-test-vm.sh precommit            # make pre-commit          (= pre-commit job)
#   ./caren-test-vm.sh unit                 # make test               (= unit-test job)
#   ./caren-test-vm.sh lint                 # make lint               (= lint-go job)
#   ./caren-test-vm.sh helm                 # chart lint + install on kind (= lint-test-helm job)
#   ./caren-test-vm.sh e2e-docker           # Docker-provider e2e     (= e2e-* jobs)
#   ./caren-test-vm.sh pre-pr               # precommit + unit + lint
#   ./caren-test-vm.sh clean                # delete kind clusters, prune docker
#
# Requirements:
#   - Ubuntu 22.04/24.04 x86_64 VM, user with passwordless sudo
#   - >= 8 vCPU / 32 GiB RAM / 200 GiB disk (the e2e and helm suites need kind)

set -euo pipefail

REPO_DIR="${CAREN_REPO_DIR:-$HOME/caren}"
REPO_URL="${CAREN_REPO_URL:-https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix.git}"
NIX_PROFILE_SH="/etc/profile.d/nix.sh"
# Match .github/workflows/checks.yml lint-test-helm: kind.create writes the
# kubeconfig to KIND_KUBECONFIG, and ct install reads it from there.
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-chart-testing}"
KIND_KUBECONFIG="${KIND_KUBECONFIG:-ct-kind-kubeconfig}"

log() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
die() {
  printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2
  exit 1
}

# ------------------------------------------------------------------------------
# Environment helpers
# ------------------------------------------------------------------------------
load_nix() {
  if ! command -v nix >/dev/null 2>&1; then
    [ -f "$NIX_PROFILE_SH" ] && . "$NIX_PROFILE_SH"
    [ -f /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh ] &&
      . /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh
  fi
  return 0
}

load_devbox() {
  command -v devbox >/dev/null 2>&1 || export PATH="$HOME/.local/bin:/usr/local/bin:$PATH"
}

github_token() {
  # CAREN itself has no private module dependencies, but a token avoids
  # anonymous GitHub API rate limits during tool downloads.
  if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
    GH_TOKEN="$(gh auth token)"
    export GH_TOKEN
  fi
  return 0
}

ensure_docker_group() {
  if ! docker info >/dev/null 2>&1; then
    if id -nG "$USER" | grep -qw docker && [ -z "${_CTV_REEXEC:-}" ]; then
      exec sg docker -c "_CTV_REEXEC=1 $(printf '%q ' "$0" "$@")"
    fi
    die "Docker daemon not reachable. Run '$0 setup', then log out and back in."
  fi
}

# goreleaser/ko build linux/arm64 images as well as amd64. A binfmt handler
# registered by a container does not survive later package installs (systemd
# re-registers only /etc/binfmt.d/ entries), so ensure it at point of use.
ensure_binfmt() {
  if [ ! -f /proc/sys/fs/binfmt_misc/qemu-aarch64 ]; then
    log "Registering QEMU arm64 binfmt handler"
    docker run --privileged --rm tonistiigi/binfmt --install arm64 >/dev/null
  fi
}

in_repo() {
  [ -d "$REPO_DIR/.git" ] || die "CAREN repo not found at $REPO_DIR. Run: $0 clone [branch]"
  cd "$REPO_DIR"
}

devbox_make() {
  in_repo
  load_nix
  load_devbox
  github_token
  log "devbox run -- make $*"
  devbox run -- make "$@"
}

# ------------------------------------------------------------------------------
# setup
# ------------------------------------------------------------------------------
cmd_setup() {
  [ "$(uname -s)" = "Linux" ] || die "Run this on the Linux VM, not your laptop"
  grep -qiE 'ubuntu|debian' /etc/os-release || die "This script supports Ubuntu/Debian"
  sudo -n true 2>/dev/null || die "Passwordless sudo required"

  # Nutanix dev VLANs block outbound port 80 and archive.ubuntu.com entirely,
  # so the stock apt sources hang forever. Switch to reachable HTTPS mirrors.
  if ! curl -sI -m 8 http://archive.ubuntu.com/ubuntu/dists/noble/Release -o /dev/null 2>/dev/null; then
    log "Port 80 / archive.ubuntu.com blocked - switching apt to HTTPS mirrors"
    for f in /etc/apt/sources.list.d/ubuntu.sources /etc/apt/sources.list; do
      sudo sed -i -E \
        -e 's|http://(archive\|us.archive)\.ubuntu\.com/ubuntu|https://us.archive.ubuntu.com/ubuntu|g' \
        -e 's|http://security\.ubuntu\.com/ubuntu|https://security.ubuntu.com/ubuntu|g' \
        "$f" 2>/dev/null || true
    done
  fi

  log "Installing base packages"
  sudo apt-get update -q
  # libatomic1: devbox's node needs libatomic.so.1, which the GitHub runner image
  # ships but a bare Ubuntu cloud image does not. Without it the markdownlint
  # prek hook fails to install ("npm install" exits 127).
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -q \
    make git curl ca-certificates gnupg unzip xz-utils openssh-client jq libatomic1

  log "Installing Docker Engine"
  command -v docker >/dev/null 2>&1 || curl -fsSL https://get.docker.com | sudo sh
  sudo systemctl enable --now docker
  sudo usermod -aG docker "$USER"

  log "Configuring inotify limits for kind"
  sudo tee /etc/sysctl.d/99-kind-inotify.conf >/dev/null <<'EOF'
fs.inotify.max_user_watches=524288
fs.inotify.max_user_instances=512
EOF
  sudo sysctl --system >/dev/null

  log "Installing gh CLI"
  if ! command -v gh >/dev/null 2>&1; then
    sudo mkdir -p -m 755 /etc/apt/keyrings
    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg |
      sudo tee /etc/apt/keyrings/githubcli-archive-keyring.gpg >/dev/null
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" |
      sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null
    sudo apt-get update -q && sudo apt-get install -y -q gh
  fi

  log "Installing Nix (daemon mode)"
  if [ ! -d /nix ]; then
    curl -fsSL https://nixos.org/nix/install -o /tmp/nix-install.sh
    sh /tmp/nix-install.sh --daemon --yes
  fi
  load_nix
  command -v nix >/dev/null 2>&1 || die "nix not on PATH after install - open a new shell and re-run"

  log "Installing devbox"
  command -v devbox >/dev/null 2>&1 || curl -fsSL https://get.jetify.com/devbox | bash -s -- -f
  load_devbox
  log "Setup complete"
}

# ------------------------------------------------------------------------------
# clone / sync-local
# ------------------------------------------------------------------------------
cmd_clone() {
  load_nix
  load_devbox
  github_token
  if [ ! -d "$REPO_DIR/.git" ]; then
    log "Cloning CAREN into $REPO_DIR"
    git clone "$REPO_URL" "$REPO_DIR"
  fi
  cd "$REPO_DIR"
  git fetch origin --tags --prune
  if [ -n "${1:-}" ]; then
    git checkout "$1"
    git pull --ff-only || true
  fi
  log "Warming devbox environment (first run downloads the whole toolchain)"
  devbox run -- bash -c 'go version && golangci-lint --version >/dev/null && echo devbox OK'
}

# Reproduce the developer's LOCAL working tree. The orchestrator leaves a git
# bundle of unpushed commits, a patch of tracked edits and a tarball of
# untracked files in ~/localsync; base history still comes from origin.
cmd_sync_local() {
  local sha="${1:?usage: $0 sync-local <sha>}"
  local x="$HOME/localsync"
  load_nix
  load_devbox
  github_token
  if [ ! -d "$REPO_DIR/.git" ]; then
    log "Cloning CAREN base history into $REPO_DIR"
    git clone "$REPO_URL" "$REPO_DIR"
  fi
  cd "$REPO_DIR"
  git fetch origin --tags --prune --quiet

  if [ -s "$x/local.bundle" ]; then
    log "Applying local commits (not on origin)"
    git fetch --quiet "$x/local.bundle" "HEAD:refs/heads/devvm-local" --force
    git checkout --quiet --force devvm-local
  else
    git checkout --quiet --force "$sha"
  fi
  git reset --hard --quiet HEAD

  if [ -s "$x/uncommitted.patch" ]; then
    log "Applying uncommitted changes"
    git apply --whitespace=nowarn "$x/uncommitted.patch"
  fi
  if [ -s "$x/untracked.tgz" ]; then
    log "Restoring untracked files"
    tar -xzf "$x/untracked.tgz" -C "$REPO_DIR"
  fi
  log "Working tree ready: $(git rev-parse --short HEAD)$(git diff --quiet || echo ' + local edits')"

  log "Warming devbox environment (first run downloads the whole toolchain)"
  devbox run -- bash -c 'go version && golangci-lint --version >/dev/null && echo devbox OK'
}

# ------------------------------------------------------------------------------
# suites (mirroring .github/workflows/checks.yml)
# ------------------------------------------------------------------------------
cmd_precommit() {
  in_repo
  load_nix
  load_devbox
  github_token
  # Same skips CI uses: those hooks have dedicated jobs.
  log "devbox run -- make pre-commit"
  devbox run -- env SKIP=no-commit-to-branch,golangci-lint,actionlint-system make pre-commit
}

cmd_unit() { devbox_make test; }
cmd_lint() { devbox_make lint; }

cmd_helm() { # lint-test-helm job
  ensure_docker_group "$@"
  ensure_binfmt
  in_repo
  load_nix
  load_devbox
  github_token
  # CI only does the expensive half when a chart actually changed; mirror that so
  # this suite costs ~a minute on the majority of PRs. CT_FORCE=true overrides.
  if [ "${CT_FORCE:-false}" != true ] &&
    [ -z "$(devbox run -- ct list-changed --config charts/ct-config.yaml 2>/dev/null)" ]; then
    log "No chart changes - skipping chart lint/install (same as CI)"
    return 0
  fi

  log "chart-testing: lint, then install onto a kind cluster"
  # Quoted heredoc: nothing expands here, it all runs inside devbox on the VM.
  # KIND_CLUSTER_NAME is passed through the environment rather than interpolated,
  # which keeps the quoting intact.
  cat >/tmp/caren-helm.sh <<'INNER'
set -euxo pipefail
ct lint --config charts/ct-config.yaml
# kind.create is a no-op when a cluster of this name already exists, and then no
# kubeconfig is written and clusterctl/ct fail. CI always starts from a fresh
# runner; on a reused VM, start from a known state.
kind delete cluster --name "${KIND_CLUSTER_NAME}" >/dev/null 2>&1 || true
make kind.create
make release-snapshot
tag="$(gojq -r .version dist/metadata.json)-$(go env GOARCH)"
kind load docker-image --name "${KIND_CLUSTER_NAME}" \
  "ko.local/cluster-api-runtime-extensions-nutanix:${tag}" \
  "ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-helm-chart-bundle-initializer:${tag}"
make clusterctl.init
KUBECONFIG="${KIND_KUBECONFIG}" ct install --config charts/ct-config.yaml \
  --helm-extra-set-args "--set-string image.repository=ko.local/cluster-api-runtime-extensions-nutanix --set-string image.tag=${tag} --set-string helmRepository.images.bundleInitializer.tag=${tag}"
INNER
  KIND_CLUSTER_NAME="$KIND_CLUSTER_NAME" KIND_KUBECONFIG="$KIND_KUBECONFIG" \
    devbox run -- bash /tmp/caren-helm.sh
  # kind.delete reads KIND_CLUSTER_NAME too - without it the chart-testing
  # cluster would be left running.
  KIND_CLUSTER_NAME="$KIND_CLUSTER_NAME" KIND_KUBECONFIG="$KIND_KUBECONFIG" \
    devbox run -- make kind.delete || true
}

cmd_e2e_docker() { # e2e-quick-start / e2e-self-hosted, Docker provider
  ensure_docker_group "$@"
  ensure_binfmt
  local focus="${E2E_FOCUS:-Quick start}"
  in_repo
  load_nix
  load_devbox
  github_token
  log "devbox run -- make e2e-test (provider:Docker, focus: $focus)"
  devbox run -- make e2e-test E2E_LABEL='provider:Docker' E2E_FOCUS="$focus" E2E_VERBOSE=true
}

cmd_clean() {
  in_repo
  load_nix
  load_devbox
  devbox run -- bash -c 'for c in $(kind get clusters 2>/dev/null); do kind delete cluster --name "$c"; done' || true
  docker system prune -f >/dev/null 2>&1 || true
}

cmd_pre_pr() {
  cmd_precommit
  cmd_unit
  cmd_lint
}

usage() { sed -n '3,25p' "$0" | sed 's/^# \{0,1\}//'; }

case "${1:-}" in
setup)
  shift
  cmd_setup "$@"
  ;;
clone)
  shift
  cmd_clone "$@"
  ;;
sync-local)
  shift
  cmd_sync_local "$@"
  ;;
precommit)
  shift
  cmd_precommit "$@"
  ;;
unit)
  shift
  cmd_unit "$@"
  ;;
lint)
  shift
  cmd_lint "$@"
  ;;
helm)
  shift
  cmd_helm "$@"
  ;;
e2e-docker)
  shift
  cmd_e2e_docker "$@"
  ;;
pre-pr)
  shift
  cmd_pre_pr "$@"
  ;;
clean)
  shift
  cmd_clean "$@"
  ;;
*)
  usage
  exit 1
  ;;
esac
