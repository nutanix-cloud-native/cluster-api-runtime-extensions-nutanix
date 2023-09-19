#!/usr/bin/env bash

# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

readonly EXAMPLES_KUSTOMIZATION_FILE="${SCRIPT_DIR}/kustomization.yaml"
readonly DOCKER_KUSTOMIZATION_FILE="${SCRIPT_DIR}/bases/docker/kustomization.yaml"
readonly AWS_KUSTOMIZATION_FILE="${SCRIPT_DIR}/bases/aws/kustomization.yaml"

trap 'find "${SCRIPT_DIR}" -name kustomization.yaml -delete' EXIT
# download the quick-start files that match the clusterctl version
envsubst <"${DOCKER_KUSTOMIZATION_FILE}.tmpl" >"${DOCKER_KUSTOMIZATION_FILE}"
envsubst <"${AWS_KUSTOMIZATION_FILE}.tmpl" >"${AWS_KUSTOMIZATION_FILE}"

# replace the kubernetes version
envsubst -no-unset <"${EXAMPLES_KUSTOMIZATION_FILE}.tmpl" >"${EXAMPLES_KUSTOMIZATION_FILE}"

mkdir -p examples/capi-quick-start
# Sync ClusterClasses (including Templates) and Clusters to separate files
kustomize build ./hack/examples |
  tee >(gojq --yaml-input --yaml-output '. | select(.metadata.labels["cluster.x-k8s.io/provider"] == "docker" and .kind != "Cluster")' >examples/capi-quick-start/docker-cluster-class.yaml) \
    >(gojq --yaml-input --yaml-output '. | select(.metadata.labels["cluster.x-k8s.io/provider"] == "docker" and .kind == "Cluster")' >examples/capi-quick-start/docker-cluster.yaml) \
    >(gojq --yaml-input --yaml-output '. | select(.metadata.labels["cluster.x-k8s.io/provider"] == "aws" and .kind != "Cluster")' >examples/capi-quick-start/aws-cluster-class.yaml) \
    >(gojq --yaml-input --yaml-output '. | select(.metadata.labels["cluster.x-k8s.io/provider"] == "aws" and .kind == "Cluster")' >examples/capi-quick-start/aws-cluster.yaml) \
    >/dev/null
