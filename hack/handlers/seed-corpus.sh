#!/usr/bin/env bash

# Copyright 2026 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# Writes golden test inputs that do not exist yet: one per example Cluster, each combined with its provider's default
# ClusterClass. Existing inputs are never overwritten because released golden files depend on them.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
readonly REPO_ROOT
cd "${REPO_ROOT}"

readonly CLUSTERCLASSES_DIR=charts/cluster-api-runtime-extensions-nutanix/defaultclusterclasses
readonly CORPUS_DIR=pkg/handlers/golden/testdata/corpus

export CLUSTER_NAME=golden
export CONTROL_PLANE_MACHINE_COUNT=1
export WORKER_MACHINE_COUNT=1
export KUBERNETES_VERSION=v1.35.3
export CONTROL_PLANE_ENDPOINT_IP=192.0.2.10
export KUBERNETES_SERVICE_LOAD_BALANCER_IP=192.0.2.20
export DOCKER_HUB_USERNAME=golden-user
export DOCKER_HUB_PASSWORD=golden-password
export AMI_LOOKUP_ORG=123456789012
export AMI_LOOKUP_FORMAT='konvoy-ami-{{.BaseOS}}-release-?{{.K8sVersion}}-*'
export AMI_LOOKUP_BASEOS=rocky-9.1
export NUTANIX_ENDPOINT=prism-central.example.com
export NUTANIX_PORT=9440
export NUTANIX_INSECURE=false
export NUTANIX_USER=golden-user
export NUTANIX_PASSWORD=golden-password
export NUTANIX_PRISM_ELEMENT_CLUSTER_NAME=golden-pe
export NUTANIX_SUBNET_NAME=golden-subnet
export NUTANIX_STORAGE_CONTAINER_NAME=golden-container
export NUTANIX_MACHINE_TEMPLATE_BASE_OS=rocky-9.7
export NUTANIX_MACHINE_TEMPLATE_LOOKUP_FORMAT='nkp-{{.BaseOS}}-release-cis-{{.K8sVersion}}-*'
export NUTANIX_FLOW_DOCKER_HUB_USERNAME=golden-user
export NUTANIX_FLOW_DOCKER_HUB_TOKEN=golden-token

seed() {
  local -r provider="${1}" cluster="${2}" out="${CORPUS_DIR}/${1}/${3}.yaml"
  [ -e "${out}" ] && return
  mkdir -p "$(dirname "${out}")"
  {
    cat "${CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml"
    for f in "${cluster}" "${SCRIPT_DIR}"/corpus-fixtures/*.yaml; do
      echo "---"
      envsubst -no-unset -no-empty -i "${f}"
    done
  } >"${out}"
  echo "seeded ${out}"
}

for f in examples/capi-quick-start/*.yaml; do
  name="$(basename "${f}" .yaml)"
  seed "${name%%-*}" "${f}" "${name}"
done

for f in "${SCRIPT_DIR}"/corpus-minimal/*.yaml; do
  seed "$(basename "${f}" .yaml)" "${f}" minimal
done
