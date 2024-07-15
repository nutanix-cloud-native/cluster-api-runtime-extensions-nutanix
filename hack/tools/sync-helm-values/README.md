<!--
 Copyright 2023 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Sync Helm Values

## Motivation

The purpose of this tool is to synchronize the helm values files located `hack/addons/kustomize` with the config maps
present in `charts/cluster-api-runtime-extensions-nutanix/templates/`, which are used by CAREN to deploy
[CAAPH](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm) `HelmChartProxy` objects.

For example the values in `hack/addons/kustomize/nfd/manifests/helm-values.yaml` get copied to the config map at
`charts/templates/cluster-api-runtime-extensions-nutanix/templates/nfd/manifests/helm-addon-installation.yaml`.

The tool sets the files located in `hack/addons/kustomize` to serve as a single source of truth for
`ClusterResourceSets` that are generated as well as the helm charts deployed.

## How it works

This tool works by traversing the file tree located at `hack/addons/kustomize` and looking for files named
`helm-values.yaml` once this file is found the corresponding config map in
`charts/templates/cluster-api-runtime-extensions-nutanix/templates/` is updated with values from `helm-values.yaml`.

