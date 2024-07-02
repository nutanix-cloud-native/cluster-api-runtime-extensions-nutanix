#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

if [ -z "${CLUSTER_AUTOSCALER_VERSION:-}" ]; then
  echo "Missing argument: CLUSTER_AUTOSCALER_VERSION"
  exit 1
fi

ASSETS_DIR="$(mktemp -d -p "${TMPDIR:-/tmp}")"
readonly ASSETS_DIR
trap_add "rm -rf ${ASSETS_DIR}" EXIT

readonly FILE_NAME="cluster-autoscaler.yaml"

readonly KUSTOMIZE_BASE_DIR="${SCRIPT_DIR}/kustomize/cluster-autoscaler/"
envsubst -no-unset -i "${KUSTOMIZE_BASE_DIR}/kustomization.yaml.tmpl" -o "${ASSETS_DIR}/kustomization.yaml"
cp "${KUSTOMIZE_BASE_DIR}"/*.yaml "${ASSETS_DIR}"

kustomize build --enable-helm "${ASSETS_DIR}" >"${ASSETS_DIR}/${FILE_NAME}"

kubectl create configmap "{{ .Values.hooks.clusterAutoscaler.crsStrategy.defaultInstallationConfigMap.name }}" --dry-run=client --output yaml \
  --from-file "${ASSETS_DIR}/${FILE_NAME}" \
  >"${ASSETS_DIR}/cluster-autoscaler-configmap.yaml"

# add warning not to edit file directly
cat <<EOF >"${GIT_REPO_ROOT}/charts/cluster-api-runtime-extensions-nutanix/templates/cluster-autoscaler/manifests/cluster-autoscaler-configmap.yaml"
$(cat "${GIT_REPO_ROOT}/hack/license-header.yaml.txt")

#=================================================================
#                 DO NOT EDIT THIS FILE
#  IT HAS BEEN GENERATED BY /hack/addons/cluster-autoscaler.sh
#=================================================================
$(cat "${ASSETS_DIR}/cluster-autoscaler-configmap.yaml")
EOF
