<!--
 Copyright 2023 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Sync Helm Values

## Motivation

The purpose of this tool is to synchronize the helm values files located `hack/addons/kustomize` with the files 
present in `charts/cluster-api-runtime-extensions-nutanix/addons/`
and `charts/cluster-api-runtime-extensions-nutanix/templates` which are used by CAREN to deploy
[CAAPH](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm) `HelmChartProxy` objects.

For example the file in `hack/addons/kustomize/nfd/manifests/helm-addon-installation.yaml` gets synced to
`charts/cluster-api-runtime-extensions-nutanix/templates/nfd/manifests/helm-addon-installation.yaml` and the
`hack/addons/kustomize/nfd/manifests/values-template.yaml` get synced to 
`charts/cluster-api-runtime-extensions-nutanix/addons/nfd/values-template.yaml`.

The tool sets the files located in `hack/addons/kustomize` to serve as a single source of truth for
`ClusterResourceSets` that are generated as well as the helm charts deployed.

## How it works

This tool works by traversing the file tree located at `hack/addons/kustomize` and looking for files named
`helm-addon-installation.yaml` and `*-template.yaml` and moves them to
`charts/cluster-api-runtime-extensions-nutanix/templates` and
`charts/cluster-api-runtime-extensions-nutanix/addons` respectively.

## Usage

This program can be invoked by go run from the `hack/tools/sync-helm-values` directory with the following command:

```bash
go run sync-values.go \
    -kustomize-directory=../../addons/kustomize/ \
    -helm-chart-directory=../../../charts/cluster-api-runtime-extensions-nutanix/
```

This program requires two flags `kustomize-directory` which is the path to the directory containing `helm-values.yaml`
files and `helm-chart-directory` which is the path to the helm chart directory for CAREN.
