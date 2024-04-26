#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

AWS_CCM_VERSION=$1
export AWS_CCM_VERSION
AWS_CCM_CHART_VERSION=$2
export AWS_CCM_CHART_VERSION

if [ -z "${AWS_CCM_VERSION:-}" ]; then
  echo "Missing argument: AWS_CCM_VERSION"
  exit 1
fi

ASSETS_DIR="$(mktemp -d -p "${TMPDIR:-/tmp}")"
readonly ASSETS_DIR
trap_add "rm -rf ${ASSETS_DIR}" EXIT

readonly KUSTOMIZE_BASE_DIR="${SCRIPT_DIR}/kustomize/aws-ccm/"
envsubst -no-unset <"${KUSTOMIZE_BASE_DIR}/kustomization.yaml.tmpl" >"${ASSETS_DIR}/kustomization.yaml"
cp "${KUSTOMIZE_BASE_DIR}"/*.yaml "${ASSETS_DIR}"

readonly FILE_NAME="aws-ccm-${AWS_CCM_VERSION}.yaml"
kustomize build --enable-helm "${ASSETS_DIR}" >"${ASSETS_DIR}/${FILE_NAME}"

kubectl create configmap aws-ccm-"${AWS_CCM_VERSION}" --dry-run=client --output yaml \
  --from-file "${ASSETS_DIR}/${FILE_NAME}" \
  >"${ASSETS_DIR}/aws-ccm-${AWS_CCM_VERSION}-configmap.yaml"

# add warning not to edit file directly
cat <<EOF >"${GIT_REPO_ROOT}/charts/cluster-api-runtime-extensions-nutanix/templates/ccm/aws/manifests/aws-ccm-${AWS_CCM_VERSION}-configmap.yaml"
$(cat "${GIT_REPO_ROOT}/hack/license-header.yaml.txt")

#=================================================================
#                 DO NOT EDIT THIS FILE
#  IT HAS BEEN GENERATED BY /hack/addons/update-aws-ccm.sh
#=================================================================
$(cat "${ASSETS_DIR}/aws-ccm-${AWS_CCM_VERSION}-configmap.yaml")
EOF
