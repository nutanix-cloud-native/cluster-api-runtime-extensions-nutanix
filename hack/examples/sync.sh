#!/usr/bin/env bash

# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

trap 'find "${SCRIPT_DIR}" -name kustomization.yaml -delete' EXIT

# For details why the exec command is structured like this , see
# https://www.shellcheck.net/wiki/SC2156.
find "${SCRIPT_DIR}" -name kustomization.yaml.tmpl \
  -exec sh -c 'k="${1}"; envsubst -no-unset -i "${k}" -o "${k%.tmpl}"' shell {} \;

readonly EXAMPLE_CLUSTERCLASSES_DIR=charts/cluster-api-runtime-extensions-nutanix/defaultclusterclasses
mkdir -p "${EXAMPLE_CLUSTERCLASSES_DIR}"
readonly EXAMPLE_CLUSTERS_DIR=examples/capi-quick-start
mkdir -p "${EXAMPLE_CLUSTERS_DIR}"

for provider in "aws" "docker" "nutanix"; do
  kustomize build --load-restrictor LoadRestrictionsNone \
    ./hack/examples/overlays/clusterclasses/"${provider}" >"${EXAMPLE_CLUSTERCLASSES_DIR}"/"${provider}"-cluster-class.yaml

  for cni in "calico" "cilium"; do
    for strategy in "helm-addon" "crs"; do
      kustomize build --load-restrictor LoadRestrictionsNone \
        ./hack/examples/overlays/clusters/"${provider}"/"${cni}"/"${strategy}" \
        >"${EXAMPLE_CLUSTERS_DIR}/${provider}-cluster-${cni}-${strategy}.yaml"
    done
  done
done

# TODO Remove once kustomize supports retaining quotes in what will be numeric values.
#shellcheck disable=SC2016
sed -i'' 's/${AMI_LOOKUP_ORG}/"${AMI_LOOKUP_ORG}"/' "${EXAMPLE_CLUSTERS_DIR}"/*.yaml

# TODO Remove once kustomize supports retaining quotes in what will be numeric values.
#shellcheck disable=SC2016
sed -i'' 's/\( cluster.x-k8s.io\/cluster-api-autoscaler-node-group-\(min\|max\)-size\): \(${WORKER_MACHINE_COUNT}\)/\1: "\3"/' "${EXAMPLE_CLUSTERS_DIR}"/*.yaml
