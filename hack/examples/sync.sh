#!/usr/bin/env bash

# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

trap 'find "${SCRIPT_DIR}" -name kustomization.yaml -delete; find "${SCRIPT_DIR}" -name "patch-*.yaml" -delete' EXIT

KUBE_VIP_CONTENT=$(cat hack/examples/files/kube-vip.yaml)
export KUBE_VIP_CONTENT

# For details why the exec command is structured like this , see
# https://www.shellcheck.net/wiki/SC2156.
# Exclude release/ so we do not require CAREN_RELEASE_VERSION and CC_TEMPLATE (used only by release).
find "${SCRIPT_DIR}" \( -name kustomization.yaml.tmpl -o -name "patch-*.yaml.tmpl" \) ! -path "${SCRIPT_DIR}/release/*" \
  -exec sh -c 'k="${1}"; envsubst -no-unset -i "${k}" -o "${k%.tmpl}"' shell {} \;

readonly EXAMPLE_CLUSTERCLASSES_DIR=charts/cluster-api-runtime-extensions-nutanix/defaultclusterclasses
readonly EXAMPLE_CLUSTERS_DIR=examples/capi-quick-start
mkdir -p "${EXAMPLE_CLUSTERCLASSES_DIR}" "${EXAMPLE_CLUSTERS_DIR}"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
readonly REPO_ROOT

# Determine Nutanix credentials overlay once for all Nutanix examples.
# Precedence:
# 1) Explicit credentials type if set
# 2) API key when provided
# 3) Basic auth when both username and password are provided
# 4) Fall back to basic-auth overlay for backward-compatible templates
NUTANIX_CREDENTIALS_OVERLAY="basic-auth"
if [[ ${NUTANIX_CREDENTIALS_TYPE:-} == "api_key" ]]; then
  NUTANIX_CREDENTIALS_OVERLAY="api-key"
elif [[ -n ${NUTANIX_CREDENTIALS_TYPE:-} && ${NUTANIX_CREDENTIALS_TYPE} != "basic_auth" ]]; then
  echo "Unsupported NUTANIX_CREDENTIALS_TYPE=${NUTANIX_CREDENTIALS_TYPE}; expected basic_auth or api_key" >&2
  exit 1
elif [[ -n ${NUTANIX_API_KEY:-} ]]; then
  NUTANIX_CREDENTIALS_OVERLAY="api-key"
fi
readonly NUTANIX_CREDENTIALS_OVERLAY

NUTANIX_USE_AUTH_OVERLAYS="${NUTANIX_USE_AUTH_OVERLAYS:-false}"
if [[ ${NUTANIX_USE_AUTH_OVERLAYS} != "true" && ${NUTANIX_USE_AUTH_OVERLAYS} != "false" ]]; then
  echo "Unsupported NUTANIX_USE_AUTH_OVERLAYS=${NUTANIX_USE_AUTH_OVERLAYS}; expected true or false" >&2
  exit 1
fi
readonly NUTANIX_USE_AUTH_OVERLAYS

if [[ ${NUTANIX_USE_AUTH_OVERLAYS} == "true" ]]; then
  echo "Setting Nutanix auth method to ${NUTANIX_CREDENTIALS_OVERLAY} for all Nutanix examples"
else
  echo "Nutanix auth overlays disabled; using classic nutanix overlays"
fi

for provider in "aws" "docker" "nutanix"; do
  kustomize build --load-restrictor LoadRestrictionsNone \
    ./hack/examples/overlays/clusterclasses/"${provider}" >"${EXAMPLE_CLUSTERCLASSES_DIR}"/"${provider}"-cluster-class.yaml
  (cd "${REPO_ROOT}" && CGO_ENABLED=0 go run ./hack/tools/clusterclass-v1beta2/main.go) <"${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml" >"${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml.tmp" && mv "${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml.tmp" "${EXAMPLE_CLUSTERCLASSES_DIR}/${provider}-cluster-class.yaml"

  for cni in "calico" "cilium"; do
    for strategy in "helm-addon" "crs"; do
      cluster_overlay="./hack/examples/overlays/clusters/${provider}/${cni}/${strategy}"
      # Keep basic-auth generation on the original nutanix overlays to preserve
      # stable document/resource ordering in generated examples.
      if [[ ${provider} == "nutanix" && ${NUTANIX_USE_AUTH_OVERLAYS} == "true" ]]; then
        cluster_overlay="./hack/examples/overlays/clusters/nutanix-auth/${NUTANIX_CREDENTIALS_OVERLAY}/${cni}/${strategy}"
      fi
      kustomize build --load-restrictor LoadRestrictionsNone \
        "${cluster_overlay}" \
        >"${EXAMPLE_CLUSTERS_DIR}/${provider}-cluster-${cni}-${strategy}.yaml"
    done
  done
done

# shellcheck disable=SC2043 # Keep the loop for future use.
for provider in "nutanix"; do
  for modifier in "failuredomains"; do
    for cni in "cilium"; do
      for strategy in "helm-addon" "crs"; do
        cluster_overlay="./hack/examples/overlays/clusters/${provider}-with-${modifier}/${cni}/${strategy}"
        if [[ ${NUTANIX_USE_AUTH_OVERLAYS} == "true" ]]; then
          cluster_overlay="./hack/examples/overlays/clusters/nutanix-auth/${NUTANIX_CREDENTIALS_OVERLAY}/with-${modifier}/${cni}/${strategy}"
        fi
        kustomize build --load-restrictor LoadRestrictionsNone \
          "${cluster_overlay}" \
          >"${EXAMPLE_CLUSTERS_DIR}/${provider}-cluster-with-${modifier}-${cni}-${strategy}.yaml"
      done
    done
  done

  # Flow is only supported on Nutanix for now and only with the helm-addon strategy.
  flow_overlay="./hack/examples/overlays/clusters/${provider}/flow/helm-addon"
  if [[ ${NUTANIX_USE_AUTH_OVERLAYS} == "true" ]]; then
    flow_overlay="./hack/examples/overlays/clusters/nutanix-auth/${NUTANIX_CREDENTIALS_OVERLAY}/flow/helm-addon"
  fi
  kustomize build --load-restrictor LoadRestrictionsNone \
    "${flow_overlay}" \
    >"${EXAMPLE_CLUSTERS_DIR}/${provider}-cluster-flow-helm-addon.yaml"

  # Nutanix project-aware cilium + helm-addon flavor.
  project_overlay="./hack/examples/overlays/clusters/${provider}/cilium/helm-addon-with-project"
  if [[ ${NUTANIX_USE_AUTH_OVERLAYS} == "true" ]]; then
    project_overlay="./hack/examples/overlays/clusters/nutanix-auth/${NUTANIX_CREDENTIALS_OVERLAY}/cilium/helm-addon-with-project"
  fi
  kustomize build --load-restrictor LoadRestrictionsNone \
    "${project_overlay}" \
    >"${EXAMPLE_CLUSTERS_DIR}/${provider}-cluster-cilium-helm-addon-with-project.yaml"

  # Nutanix project-scoped-user flavor: same cluster as above, but all Prism Central
  # credentials come from a user scoped to NUTANIX_PROJECT_NAME. Project-scoped users
  # authenticate with username/password, so this flavor is always built from the
  # basic-auth-shaped classic overlay regardless of NUTANIX_USE_AUTH_OVERLAYS.
  kustomize build --load-restrictor LoadRestrictionsNone \
    "./hack/examples/overlays/clusters/${provider}/cilium/helm-addon-with-project-scoped-user" \
    >"${EXAMPLE_CLUSTERS_DIR}/${provider}-cluster-cilium-helm-addon-with-project-scoped-user.yaml"
done
unset provider cni strategy

# Nutanix templates use Cluster v1beta2 (patch cluster-apiversion-v1beta2.yaml) so only classRef
# is used; v1beta1 would reject classRef at apply time. Strip any legacy class line from other providers.
for f in "${EXAMPLE_CLUSTERS_DIR}"/*.yaml; do
  if [ -f "${f}" ]; then
    sed '/^    class: /d' "${f}" >"${f}.tmp" && mv "${f}.tmp" "${f}"
  fi
done

# Generate EKS cluster-class and cluster templates.
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
