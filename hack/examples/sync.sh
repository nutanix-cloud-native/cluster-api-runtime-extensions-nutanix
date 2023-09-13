#!/usr/bin/env bash

# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

CAPD_KUSTOMIZATION_FILE="${SCRIPT_DIR}/bases/capd/kustomization.yaml"

# download the quick-start files that match the clusterctl version
trap 'rm -rf ${CAPD_KUSTOMIZATION_FILE}' EXIT
CLUSTERCTL_VERSION=$(clusterctl version -o short 2>/dev/null) envsubst \
  <"${SCRIPT_DIR}/bases/capd/kustomization.yaml.tmpl" >"${CAPD_KUSTOMIZATION_FILE}"

mkdir -p examples/capi-quick-start
# Sync ClusterClass and all Templates
kustomize build ./hack/examples |
  gojq --yaml-input --yaml-output '. | select(.kind != "Cluster")' >examples/capi-quick-start/capd-cluster-class.yaml
# Sync Cluster
kustomize build ./hack/examples |
  gojq --yaml-input --yaml-output '. | select(.kind == "Cluster")' >examples/capi-quick-start/capd-cluster.yaml
