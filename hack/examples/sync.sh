#!/usr/bin/env bash

# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

trap 'find "${SCRIPT_DIR}" -name kustomization.yaml -delete' EXIT

KUBE_VIP_CONTENT=$(cat hack/examples/files/kube-vip.yaml)
export KUBE_VIP_CONTENT

# For details why the exec command is structured like this , see
# https://www.shellcheck.net/wiki/SC2156.
# Exclude release/ so we do not require CAREN_RELEASE_VERSION and CC_TEMPLATE (used only by release).
find "${SCRIPT_DIR}" -name kustomization.yaml.tmpl ! -path "${SCRIPT_DIR}/release/*" \
  -exec sh -c 'k="${1}"; envsubst -no-unset -i "${k}" -o "${k%.tmpl}"' shell {} \;

readonly EXAMPLE_CLUSTERCLASSES_DIR=charts/cluster-api-runtime-extensions-nutanix/defaultclusterclasses
readonly EXAMPLE_CLUSTERS_DIR=examples/capi-quick-start
mkdir -p "${EXAMPLE_CLUSTERCLASSES_DIR}" "${EXAMPLE_CLUSTERS_DIR}"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
readonly REPO_ROOT

for provider in "aws" "docker" "nutanix"; do
  kustomize build --load-restrictor LoadRestrictionsNone \
    ./hack/examples/overlays/clusterclasses/"${provider}" >"${EXAMPLE_CLUSTERCLASSES_DIR}"/"${provider}"-cluster-class.yaml
  (cd "${REPO_ROOT}" && go run ./hack/tools/clusterclass-v1beta2/main.go) <"${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml" >"${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml.tmp" && mv "${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml.tmp" "${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml"

  for cni in "calico" "cilium"; do
    for strategy in "helm-addon" "crs"; do
      kustomize build --load-restrictor LoadRestrictionsNone \
        ./hack/examples/overlays/clusters/"${provider}"/"${cni}"/"${strategy}" \
        >"${EXAMPLE_CLUSTERS_DIR}/${provider}-cluster-${cni}-${strategy}.yaml"
    done
  done
done

# shellcheck disable=SC2043 # Keep the loop for future use.
for provider in "nutanix"; do
  for modifier in "failuredomains"; do
    for cni in "cilium"; do
      for strategy in "helm-addon" "crs"; do
        kustomize build --load-restrictor LoadRestrictionsNone \
          ./hack/examples/overlays/clusters/"${provider}"-with-"${modifier}"/"${cni}"/"${strategy}" \
          >"${EXAMPLE_CLUSTERS_DIR}/${provider}-cluster-with-${modifier}-${cni}-${strategy}.yaml"
      done
    done
  done
done
unset provider cni strategy

# Nutanix templates use Cluster v1beta2 (patch cluster-apiversion-v1beta2.yaml) so only classRef
# is used; v1beta1 would reject classRef at apply time. Strip any legacy class line from other providers.
for f in "${EXAMPLE_CLUSTERS_DIR}"/*.yaml; do
  if [ -f "${f}" ]; then
    sed '/^    class: /d' "${f}" >"${f}.tmp" && mv "${f}.tmp" "${f}"
  fi
done

kustomize build --load-restrictor LoadRestrictionsNone \
  ./hack/examples/overlays/clusterclasses/eks >"${EXAMPLE_CLUSTERCLASSES_DIR}"/eks-cluster-class.yaml
(cd "${REPO_ROOT}" && go run ./hack/tools/clusterclass-v1beta2/main.go) <"${EXAMPLE_CLUSTERCLASSES_DIR}/eks-cluster-class.yaml" >"${EXAMPLE_CLUSTERCLASSES_DIR}/eks-cluster-class.yaml.tmp" && mv "${EXAMPLE_CLUSTERCLASSES_DIR}/eks-cluster-class.yaml.tmp" "${EXAMPLE_CLUSTERCLASSES_DIR}/eks-cluster-class.yaml"
sed -i'' 's/ name: eks-eks-/ name: eks-/' "${EXAMPLE_CLUSTERCLASSES_DIR}"/eks-cluster-class.yaml

kustomize build --load-restrictor LoadRestrictionsNone \
  ./hack/examples/overlays/clusters/eks \
  >"${EXAMPLE_CLUSTERS_DIR}/eks-cluster.yaml"

# TODO Remove once kustomize supports retaining quotes in what will be numeric values.
#shellcheck disable=SC2016
sed -i'' 's/${AMI_LOOKUP_ORG}/"${AMI_LOOKUP_ORG}"/' "${EXAMPLE_CLUSTERS_DIR}"/*.yaml

# TODO Remove once kustomize supports retaining quotes in what will be numeric values.
#shellcheck disable=SC2016
sed -i'' 's/\( cluster.x-k8s.io\/cluster-api-autoscaler-node-group-\(min\|max\)-size\): \(${WORKER_MACHINE_COUNT}\)/\1: "\3"/' "${EXAMPLE_CLUSTERS_DIR}"/*.yaml
