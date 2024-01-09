<!--
 Copyright 2023 D2iQ, Inc. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# capi-runtime-extensions

![Version: v0.0.0-dev](https://img.shields.io/badge/Version-v0.0.0--dev-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.0.0-dev](https://img.shields.io/badge/AppVersion-v0.0.0--dev-informational?style=flat-square)

A Helm chart for capi-runtime-extensions

**Homepage:** <https://github.com/d2iq-labs/capi-runtime-extensions>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| jimmidyson | <jimmidyson@gmail.com> | <https://eng.d2iq.com> |

## Source Code

* <https://github.com/d2iq-labs/capi-runtime-extensions>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| certificates.issuer.kind | string | `"Issuer"` |  |
| certificates.issuer.name | string | `""` |  |
| certificates.issuer.selfSigned | bool | `true` |  |
| deployDefaultClusterClasses | bool | `true` |  |
| deployment.replicas | int | `1` |  |
| env | object | `{}` |  |
| hooks.CalicoCNI.defaultValueTemplatesConfigMaps.AWSCluster.create | bool | `true` |  |
| hooks.CalicoCNI.defaultValueTemplatesConfigMaps.AWSCluster.name | string | `"calico-cni-installation-template-awscluster"` |  |
| hooks.CalicoCNI.defaultValueTemplatesConfigMaps.DockerCluster.create | bool | `true` |  |
| hooks.CalicoCNI.defaultValueTemplatesConfigMaps.DockerCluster.name | string | `"calico-cni-installation-template-dockercluster"` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"ghcr.io/d2iq-labs/capi-runtime-extensions"` |  |
| image.tag | string | `""` |  |
| imagePullSecrets | list | `[]` | Optional secrets used for pulling the container image |
| nodeSelector | object | `{}` |  |
| priorityClassName | string | `""` | Optional priority class to be used for the pod. |
| resources.limits.cpu | string | `"100m"` |  |
| resources.limits.memory | string | `"256Mi"` |  |
| resources.requests.cpu | string | `"100m"` |  |
| resources.requests.memory | string | `"128Mi"` |  |
| securityContext.runAsUser | int | `65532` |  |
| service.annotations | object | `{}` |  |
| service.port | int | `443` |  |
| service.type | string | `"ClusterIP"` |  |
| tolerations | list | `[]` |  |
