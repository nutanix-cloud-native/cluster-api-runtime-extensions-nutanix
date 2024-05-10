#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

ASSETS_DIR="$(mktemp -d -p "${TMPDIR:-/tmp}")"

cp "${GIT_REPO_ROOT}/charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml" "${ASSETS_DIR}"

# this sed line is needed because the go library is unable to parse yaml with a template string.
sed -i s/"{{ .Values.helmAddonsConfigMap }}"/placeholder/g "${ASSETS_DIR}/helm-config.yaml"
go run "${GIT_REPO_ROOT}/hack/tools/mindthegap-helm-reg/main.go" --input-configmap-file="${ASSETS_DIR}/helm-config.yaml" --output-file="${ASSETS_DIR}/repos.yaml"

# add warning not to edit file directly
cat <<EOF >"${GIT_REPO_ROOT}/hack/addons/mindthegap-helm-registry/repos.yaml"
$(cat "${GIT_REPO_ROOT}/hack/license-header.yaml.txt")

#=================================================================
#                 DO NOT EDIT THIS FILE
#  IT HAS BEEN GENERATED BY /hack/addons/generate-mindthegap-repofile.sh
#=================================================================
$(cat "${ASSETS_DIR}/repos.yaml")
EOF
