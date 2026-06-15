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

build_clusterclass() {
  local provider="${1}"
  kustomize build --load-restrictor LoadRestrictionsNone \
    ./hack/examples/overlays/clusterclasses/"${provider}" >"${EXAMPLE_CLUSTERCLASSES_DIR}"/"${provider}"-cluster-class.yaml
  (cd "${REPO_ROOT}" && CGO_ENABLED=0 go run ./hack/tools/clusterclass-v1beta2/main.go) \
    <"${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml" \
    >"${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml.tmp" &&
    mv "${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml.tmp" \
      "${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml"
}

# Build a cluster template from an overlay. Usage:
#   build_cluster <overlay-subpath> <output-basename>
# e.g. build_cluster docker/cilium/helm-addon docker-cluster-cilium-helm-addon
build_cluster() {
  local overlay="${1}"
  local basename="${2}"
  kustomize build --load-restrictor LoadRestrictionsNone \
    ./hack/examples/overlays/clusters/"${overlay}" \
    >"${EXAMPLE_CLUSTERS_DIR}/${basename}.yaml"
}

# AWS keeps the historical naming (unqualified CNI, MetalLB is the only SLB).
build_clusterclass aws
for cni in "calico" "cilium"; do
  for strategy in "helm-addon" "crs"; do
    build_cluster "aws/${cni}/${strategy}" "aws-cluster-${cni}-${strategy}"
  done
done

# Docker supports Cilium (+Cilium-SLB helm-addon, +MetalLB helm-addon/crs) and
# Calico (+MetalLB helm-addon/crs). The unqualified "cilium" name now means
# Cilium CNI + Cilium ServiceLoadBalancer; MetalLB variants are explicit.
build_clusterclass docker
build_cluster "docker/cilium/helm-addon" "docker-cluster-cilium-helm-addon"
for cni in "cilium" "calico"; do
  for strategy in "helm-addon" "crs"; do
    build_cluster "docker/${cni}-metallb/${strategy}" "docker-cluster-${cni}-metallb-${strategy}"
  done
done

# Nutanix mirrors Docker's Cilium + Calico layout, and adds Flow (MetalLB only,
# helm-addon only) plus the "with-failuredomains" variants.
build_clusterclass nutanix
build_cluster "nutanix/cilium/helm-addon" "nutanix-cluster-cilium-helm-addon"
for cni in "cilium" "calico"; do
  for strategy in "helm-addon" "crs"; do
    build_cluster "nutanix/${cni}-metallb/${strategy}" "nutanix-cluster-${cni}-metallb-${strategy}"
  done
done
build_cluster "nutanix/flow-metallb/helm-addon" "nutanix-cluster-flow-metallb-helm-addon"

build_cluster "nutanix-with-failuredomains/cilium/helm-addon" \
  "nutanix-cluster-with-failuredomains-cilium-helm-addon"
for strategy in "helm-addon" "crs"; do
  build_cluster "nutanix-with-failuredomains/cilium-metallb/${strategy}" \
    "nutanix-cluster-with-failuredomains-cilium-metallb-${strategy}"
done
unset cni strategy

# Nutanix templates use Cluster v1beta2 (patch cluster-apiversion-v1beta2.yaml) so only classRef
# is used; v1beta1 would reject classRef at apply time. Strip any legacy class line from other providers.
for f in "${EXAMPLE_CLUSTERS_DIR}"/*.yaml; do
  if [ -f "${f}" ]; then
    sed '/^    class: /d' "${f}" >"${f}.tmp" && mv "${f}.tmp" "${f}"
  fi
done

kustomize build --load-restrictor LoadRestrictionsNone \
  ./hack/examples/overlays/clusterclasses/eks >"${EXAMPLE_CLUSTERCLASSES_DIR}"/eks-cluster-class.yaml
(cd "${REPO_ROOT}" && CGO_ENABLED=0 go run ./hack/tools/clusterclass-v1beta2/main.go) <"${EXAMPLE_CLUSTERCLASSES_DIR}/eks-cluster-class.yaml" >"${EXAMPLE_CLUSTERCLASSES_DIR}/eks-cluster-class.yaml.tmp" && mv "${EXAMPLE_CLUSTERCLASSES_DIR}/eks-cluster-class.yaml.tmp" "${EXAMPLE_CLUSTERCLASSES_DIR}/eks-cluster-class.yaml"
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
