#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

# Below are the lists of CSI Providers allowed for a specific infrastructure.
# - When we support a new infrastructure, we need to a create a new entry in this array using the same convention.
# - When we support a new CSI Provider, we need to add it to one or more of these lists.
declare -rA CSI_PROVIDERS=(
  ["aws"]='["aws-ebs"]'
  ["nutanix"]='["nutanix"]'
  ["docker"]='["local-path"]'
)

readonly CSI_JSONPATH='.spec.versions[].schema.openAPIV3Schema.properties.spec.properties.addons.properties.csi.properties'

for provider in "${!CSI_PROVIDERS[@]}"; do
  custerconfig_file="${GIT_REPO_ROOT}/api/v1alpha1/crds/caren.nutanix.com_${provider}clusterconfigs.yaml"
  cat <<EOF >"${custerconfig_file}.tmp"
$(cat "${GIT_REPO_ROOT}/hack/license-header.yaml.txt")
---
$(gojq --yaml-input --yaml-output \
    "(${CSI_JSONPATH}.providers.items.properties.name.enum, ${CSI_JSONPATH}.defaultStorage.properties.providerName.enum) |= ${CSI_PROVIDERS[${provider}]}" \
    "${custerconfig_file}")
EOF

  mv "${custerconfig_file}"{.tmp,}
done
