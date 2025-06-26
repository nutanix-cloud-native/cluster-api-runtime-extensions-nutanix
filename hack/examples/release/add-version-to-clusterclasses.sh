#!/usr/bin/env bash

# Copyright 2024 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

trap 'find "${SCRIPT_DIR}" -name kustomization.yaml -delete' EXIT

export CAREN_RELEASE_VERSION="${1}"

for CC_TEMPLATE in "${SCRIPT_DIR}"/../../../charts/cluster-api-runtime-extensions-nutanix/clusterclasses/**/*.yaml; do
  export CC_TEMPLATE
  envsubst -no-empty -no-unset -i "${SCRIPT_DIR}/kustomization.yaml.tmpl" -o "${SCRIPT_DIR}/kustomization.yaml"

  kustomize build "${SCRIPT_DIR}" --load-restrictor LoadRestrictionsNone >"${SCRIPT_DIR}/$(basename "${CC_TEMPLATE}")"
done
