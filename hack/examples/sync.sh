#!/usr/bin/env bash

# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

trap 'find "${SCRIPT_DIR}" -name kustomization.yaml -delete' EXIT

find "${SCRIPT_DIR}" -name kustomization.yaml.tmpl \
  -exec bash -ec 'mkdir -p $(dirname {}) && envsubst -no-unset <"{}" >"$(dirname {})/$(basename -s .tmpl {})"' \;

readonly EXAMPLE_CLUSTERCLASSES_DIR=charts/capi-runtime-extensions/defaultclusterclasses
mkdir -p "${EXAMPLE_CLUSTERCLASSES_DIR}"
readonly EXAMPLE_CLUSTERS_DIR=examples/capi-quick-start
mkdir -p "${EXAMPLE_CLUSTERS_DIR}"

# Sync ClusterClasses (including Templates) and Clusters to separate files.
kustomize build ./hack/examples |
  tee \
    >(
      gojq --yaml-input --yaml-output 'select(.metadata.labels["cluster.x-k8s.io/provider"] == "docker"
                                        and .kind != "Cluster"
                                        and .kind != "DockerMachinePoolTemplate"
                                      )' >"${EXAMPLE_CLUSTERCLASSES_DIR}/docker-cluster-class.yaml"
    ) \
    >(
      gojq --yaml-input --yaml-output 'select(
                                        .metadata.labels["cluster.x-k8s.io/provider"] == "docker"
                                        and .kind == "Cluster"
                                        and .spec.topology.variables[0].value.addons.cni.provider == "Cilium"
                                        and .spec.topology.variables[0].value.addons.cni.strategy == "ClusterResourceSet"
                                      )' >"${EXAMPLE_CLUSTERS_DIR}/docker-cluster-cilium-crs.yaml"
    ) \
    >(
      gojq --yaml-input --yaml-output 'select(
                                        .metadata.labels["cluster.x-k8s.io/provider"] == "docker"
                                        and .kind == "Cluster"
                                        and .spec.topology.variables[0].value.addons.cni.provider == "Cilium"
                                        and .spec.topology.variables[0].value.addons.cni.strategy == "HelmAddon"
                                      )' >"${EXAMPLE_CLUSTERS_DIR}/docker-cluster-cilium-helm-addon.yaml"
    ) \
    >(
      gojq --yaml-input --yaml-output 'select(
                                        .metadata.labels["cluster.x-k8s.io/provider"] == "docker"
                                        and .kind == "Cluster"
                                        and .spec.topology.variables[0].value.addons.cni.provider == "Calico"
                                        and .spec.topology.variables[0].value.addons.cni.strategy == "ClusterResourceSet"
                                      )' >"${EXAMPLE_CLUSTERS_DIR}/docker-cluster-calico-crs.yaml"
    ) \
    >(
      gojq --yaml-input --yaml-output 'select(
                                        .metadata.labels["cluster.x-k8s.io/provider"] == "docker"
                                        and .kind == "Cluster"
                                        and .spec.topology.variables[0].value.addons.cni.provider == "Calico"
                                        and .spec.topology.variables[0].value.addons.cni.strategy == "HelmAddon"
                                      )' >"${EXAMPLE_CLUSTERS_DIR}/docker-cluster-calico-helm-addon.yaml"
    ) \
    >(
      gojq --yaml-input --yaml-output 'select(.metadata.labels["cluster.x-k8s.io/provider"] == "aws"
                                        and .kind != "Cluster"
                                        and .kind != "AWSClusterStaticIdentity"
                                      )' >"${EXAMPLE_CLUSTERCLASSES_DIR}/aws-cluster-class.yaml"
    ) \
    >(
      gojq --yaml-input --yaml-output 'select((.metadata.labels["cluster.x-k8s.io/provider"] == "aws"
                                        and .kind == "Cluster"
                                        and .spec.topology.variables[0].value.addons.cni.provider == "Calico"
                                        and .spec.topology.variables[0].value.addons.cni.strategy == "ClusterResourceSet"
                                        ) or .kind == "AWSClusterStaticIdentity")' >"${EXAMPLE_CLUSTERS_DIR}/aws-cluster-calico-crs.yaml"
    ) \
    >(
      gojq --yaml-input --yaml-output 'select((.metadata.labels["cluster.x-k8s.io/provider"] == "aws"
                                        and .kind == "Cluster"
                                        and .spec.topology.variables[0].value.addons.cni.provider == "Calico"
                                        and .spec.topology.variables[0].value.addons.cni.strategy == "HelmAddon"
                                      ) or .kind == "AWSClusterStaticIdentity")' >"${EXAMPLE_CLUSTERS_DIR}/aws-cluster-calico-helm-addon.yaml"
    ) \
    >(
      gojq --yaml-input --yaml-output 'select((.metadata.labels["cluster.x-k8s.io/provider"] == "aws"
                                        and .kind == "Cluster"
                                        and .spec.topology.variables[0].value.addons.cni.provider == "Cilium"
                                        and .spec.topology.variables[0].value.addons.cni.strategy == "ClusterResourceSet"
                                      ) or .kind == "AWSClusterStaticIdentity")' >"${EXAMPLE_CLUSTERS_DIR}/aws-cluster-cilium-crs.yaml"
    ) \
    >(
      gojq --yaml-input --yaml-output 'select((.metadata.labels["cluster.x-k8s.io/provider"] == "aws"
                                        and .kind == "Cluster"
                                        and .spec.topology.variables[0].value.addons.cni.provider == "Cilium"
                                        and .spec.topology.variables[0].value.addons.cni.strategy == "HelmAddon"
                                      ) or .kind == "AWSClusterStaticIdentity")' >"${EXAMPLE_CLUSTERS_DIR}/aws-cluster-cilium-helm-addon.yaml"
    ) \
    >/dev/null

#shellcheck disable=SC2016
sed -i'' 's/^  name: .\+$/  name: ${CLUSTER_NAME}/' "${EXAMPLE_CLUSTERS_DIR}"/*.yaml

# TODO Remove once kustomize supports retaining quotes in what will be numeric values.
#shellcheck disable=SC2016
sed -i'' 's/${AMI_LOOKUP_ORG}/"${AMI_LOOKUP_ORG}"/' "${EXAMPLE_CLUSTERS_DIR}"/*.yaml

# TODO Remove once CAPA templates default to using external cloud provider.
sed -i'' 's/cloud-provider:\ aws/cloud-provider:\ external/g' "${EXAMPLE_CLUSTERCLASSES_DIR}/aws-cluster-class.yaml"
