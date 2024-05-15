#!/usr/bin/env bash
#
# this script fetches the CRDs so that we can use the bundle type in our templates
# see this issue which describes the problem: https://github.com/cert-manager/trust-manager/issues/281
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

RAW_URL="https://raw.githubusercontent.com/cert-manager/trust-manager"
VERSION="$(yq .dependencies[0].version <"${GIT_REPO_ROOT}/charts/cluster-api-runtime-extensions-nutanix/Chart.yaml" | xargs)"
CRD_URL_PATH="deploy/charts/trust-manager/templates/crd-trust.cert-manager.io_bundles.yaml"
CRD_PATH="${GIT_REPO_ROOT}/charts/cluster-api-runtime-extensions-nutanix/crds/"
mkdir -p "${CRD_PATH}"
echo "${RAW_URL}/${VERSION}/${CRD_URL_PATH}"
CRD_FILE="${CRD_PATH}/crd-trust.cert-manager.io_bundles.yaml"
curl -l -o "${CRD_FILE}" "${RAW_URL}/${VERSION}/${CRD_URL_PATH}"
#shellcheck disable=SC1083 # this is supposed to be literal values
sed -i s/{{\.*}}//g "${CRD_FILE}"
yq -Y '.metadata.annotations["meta.helm.sh/release-name"] = "cluster-api-runtime-extensions-nutanix" |
    .metadata.annotations["meta.helm.sh/release-namespace"] = "default" |
    .metadata.labels["app.kubernetes.io/managed-by"] = "Helm"' <./charts/cluster-api-runtime-extensions-nutanix/crds/crd-trust.cert-manager.io_bundles.yaml >>tmp.yaml

mv tmp.yaml "${CRD_FILE}"
