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
| Jimmi Dyson | <jimmidyson@gmail.com> | <https://eng.d2iq.com> |

## Source Code

* <https://github.com/d2iq-labs/capi-runtime-extensions>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| certificate.issuer.kind | string | `"Issuer"` |  |
| certificate.issuer.name | string | `nil` |  |
| certificate.issuer.selfSigned | bool | `true` |  |
| env | object | `{}` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"ghcr.io/d2iq-labs/capi-runtime-extensions"` |  |
| image.tag | string | `nil` |  |
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
