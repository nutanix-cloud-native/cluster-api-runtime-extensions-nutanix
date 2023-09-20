#!/usr/bin/env bash

# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

readonly EXAMPLES_KUSTOMIZATION_FILE="${SCRIPT_DIR}/kustomization.yaml"
readonly CAPD_KUSTOMIZATION_FILE="${SCRIPT_DIR}/bases/capd/kustomization.yaml"

trap 'rm -rf ${CAPD_KUSTOMIZATION_FILE} ${EXAMPLES_KUSTOMIZATION_FILE}' EXIT
# download the quick-start files that match the clusterctl version
CLUSTERCTL_VERSION=$(clusterctl version -o short 2>/dev/null) envsubst \
  <"${CAPD_KUSTOMIZATION_FILE}.tmpl" >"${CAPD_KUSTOMIZATION_FILE}"
# replace the kubernetes version
envsubst -no-unset <"${EXAMPLES_KUSTOMIZATION_FILE}.tmpl" >"${EXAMPLES_KUSTOMIZATION_FILE}"

mkdir -p examples/capi-quick-start
# Sync ClusterClass and all Templates
kustomize build ./hack/examples |
  gojq --yaml-input --yaml-output '. | select(.kind != "Cluster")' >examples/capi-quick-start/capd-cluster-class.yaml
# Sync Cluster
kustomize build ./hack/examples |
  gojq --yaml-input --yaml-output '. | select(.kind == "Cluster")' >examples/capi-quick-start/capd-cluster.yaml
