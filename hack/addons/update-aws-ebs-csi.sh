#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

if [ -z "${AWS_EBS_CSI_CHART_VERSION:-}" ]; then
  echo "Missing environment variable: AWS_EBS_CSI_CHART_VERSION"
  exit 1
fi

ASSETS_DIR="$(mktemp -d -p "${TMPDIR:-/tmp}")"
readonly ASSETS_DIR
trap_add "rm -rf ${ASSETS_DIR}" EXIT

readonly FILE_NAME="aws-ebs-csi.yaml"

readonly KUSTOMIZE_BASE_DIR="${SCRIPT_DIR}/kustomize/aws-ebs-csi"
mkdir -p "${ASSETS_DIR}/aws-ebs-csi"
envsubst -no-unset <"${KUSTOMIZE_BASE_DIR}/kustomization.yaml.tmpl" >"${KUSTOMIZE_BASE_DIR}/kustomization.yaml"
trap_add "rm -f ${KUSTOMIZE_BASE_DIR}/kustomization.yaml" EXIT

kustomize build \
  --load-restrictor LoadRestrictionsNone \
  --enable-helm "${KUSTOMIZE_BASE_DIR}/" >"${ASSETS_DIR}/${FILE_NAME}"
trap_add "rm -rf ${KUSTOMIZE_BASE_DIR}/charts/" EXIT

kubectl create configmap aws-ebs-csi --dry-run=client --output yaml \
  --from-file "${ASSETS_DIR}/${FILE_NAME}" \
  >"${ASSETS_DIR}/aws-ebs-csi-configmap.yaml"

# add warning not to edit file directly
cat <<EOF >"${GIT_REPO_ROOT}/charts/cluster-api-runtime-extensions-nutanix/templates/csi/aws-ebs/manifests/aws-ebs-csi-configmap.yaml"
$(cat "${GIT_REPO_ROOT}/hack/license-header.yaml.txt")

#=================================================================
#                 DO NOT EDIT THIS FILE
#  IT HAS BEEN GENERATED BY /hack/addons/update-aws-ebs-csi.sh
#=================================================================
$(cat "${ASSETS_DIR}/aws-ebs-csi-configmap.yaml")
EOF
