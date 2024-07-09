#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

if [ -z "${SNAPSHOT_CONTROLLER_CHART_VERSION:-}" ]; then
  echo "Missing environment variable: SNAPSHOT_CONTROLLER_CHART_VERSION"
  exit 1
fi

ASSETS_DIR="$(mktemp -d -p "${TMPDIR:-/tmp}")"
readonly ASSETS_DIR
trap_add "rm -rf ${ASSETS_DIR}" EXIT

readonly FILE_NAME="snapshot-controller.yaml"

readonly KUSTOMIZE_BASE_DIR="${SCRIPT_DIR}/kustomize/csi/snapshot-controller/manifests"
mkdir -p "${ASSETS_DIR}/snapshot-controller"
envsubst -no-unset <"${KUSTOMIZE_BASE_DIR}/kustomization.yaml.tmpl" >"${ASSETS_DIR}/snapshot-controller/kustomization.yaml"
cp -r "${KUSTOMIZE_BASE_DIR}"/*.yaml "${ASSETS_DIR}/snapshot-controller/"

kustomize build --enable-helm "${ASSETS_DIR}/snapshot-controller/" >"${ASSETS_DIR}/${FILE_NAME}"

kubectl create configmap snapshot-controller --dry-run=client --output yaml \
  --from-file "${ASSETS_DIR}/${FILE_NAME}" \
  >"${ASSETS_DIR}/snapshot-controller-configmap.yaml"

# add warning not to edit file directly
cat <<EOF >"${GIT_REPO_ROOT}/charts/cluster-api-runtime-extensions-nutanix/templates/csi/snapshot-controller/manifests/snapshot-controller-configmap.yaml"
$(cat "${GIT_REPO_ROOT}/hack/license-header.yaml.txt")

#=================================================================
#                 DO NOT EDIT THIS FILE
#  IT HAS BEEN GENERATED BY /hack/addons/update-snapshot-controller.sh
#=================================================================
$(cat "${ASSETS_DIR}/snapshot-controller-configmap.yaml")
EOF
