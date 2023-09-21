#!/usr/bin/env bash

# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

trap 'find "${SCRIPT_DIR}" -name kustomization.yaml -delete' EXIT

find "${SCRIPT_DIR}" -name kustomization.yaml.tmpl \
  -exec bash -ec 'envsubst -no-unset <"{}" >"$(dirname {})/$(basename -s .tmpl {})"' \;

mkdir -p examples/capi-quick-start
# Sync ClusterClasses (including Templates) and Clusters to separate files
kustomize build ./hack/examples |
  tee >(gojq --yaml-input --yaml-output '. | select(.metadata.labels["cluster.x-k8s.io/provider"] == "docker" and .kind != "Cluster")' >examples/capi-quick-start/docker-cluster-class.yaml) \
    >(gojq --yaml-input --yaml-output '. | select(.metadata.labels["cluster.x-k8s.io/provider"] == "docker" and .kind == "Cluster")' >examples/capi-quick-start/docker-cluster.yaml) \
    >(gojq --yaml-input --yaml-output '. | select(.metadata.labels["cluster.x-k8s.io/provider"] == "aws" and ( .kind != "Cluster" and .kind != "AWSClusterStaticIdentity"))' >examples/capi-quick-start/aws-cluster-class.yaml) \
    >(gojq --yaml-input --yaml-output '. | select(.metadata.labels["cluster.x-k8s.io/provider"] == "aws" and ( .kind == "Cluster" or .kind == "AWSClusterStaticIdentity"))' >examples/capi-quick-start/aws-cluster.yaml) \
    >/dev/null
