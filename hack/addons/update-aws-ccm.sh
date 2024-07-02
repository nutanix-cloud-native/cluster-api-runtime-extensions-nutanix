#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

export AWS_CCM_VERSION="${1}"
export AWS_CCM_CHART_VERSION="${2}"

if [ -z "${AWS_CCM_VERSION:-}" ]; then
  echo "Missing argument: AWS_CCM_VERSION"
  exit 1
fi

if ! crane manifest "registry.k8s.io/provider-aws/cloud-controller-manager:${AWS_CCM_VERSION}" &>/dev/null; then
  echo "AWS CCM image registry.k8s.io/provider-aws/cloud-controller-manager:${AWS_CCM_VERSION} does not exist"
  echo "Check the image specified image tag"
  exit 1
fi

ASSETS_DIR="$(mktemp -d -p "${TMPDIR:-/tmp}")"
readonly ASSETS_DIR
trap_add "rm -rf ${ASSETS_DIR}" EXIT

readonly KUSTOMIZE_BASE_DIR="${SCRIPT_DIR}/kustomize/aws-ccm/"
envsubst -no-unset -i "${KUSTOMIZE_BASE_DIR}/kustomization.yaml.tmpl" -o "${ASSETS_DIR}/kustomization.yaml"
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

# Check that the versions specified in the helm chart have been updated too.
# shellcheck disable=SC2001 # Requires sed.
K8S_MINOR_VERSION="$(echo "${AWS_CCM_VERSION}" | sed -e 's/^v\([0-9]\+\.[0-9]\+\).\+$/\1/')"
if gojq --yaml-input --exit-status \
  ".hooks.ccm.aws.k8sMinorVersionToCCMVersion[\"${K8S_MINOR_VERSION}\"] != env.AWS_CCM_VERSION" \
  "${GIT_REPO_ROOT}/charts/cluster-api-runtime-extensions-nutanix/values.yaml" &>/dev/null; then
  echo "The AWS CCM version for ${K8S_MINOR_VERSION} in the helm chart is not up to date."
  echo "Please update the version in the helm chart for Kubernetes minor version ${K8S_MINOR_VERSION} to ${AWS_CCM_VERSION}"
  exit 1
fi
