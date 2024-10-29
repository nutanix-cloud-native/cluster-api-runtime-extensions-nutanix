<!--
 Copyright 2024 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# cluster-api-runtime-extensions-nutanix

![Version: v0.0.0-dev](https://img.shields.io/badge/Version-v0.0.0--dev-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.0.0-dev](https://img.shields.io/badge/AppVersion-v0.0.0--dev-informational?style=flat-square)

A Helm chart for cluster-api-runtime-extensions-nutanix

**Homepage:** <https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| jimmidyson | <jimmidyson@gmail.com> | <https://eng.d2iq.com> |

## Source Code

* <https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| certificates.issuer.kind | string | `"Issuer"` |  |
| certificates.issuer.name | string | `""` |  |
| certificates.issuer.selfSigned | bool | `true` |  |
| deployDefaultClusterClasses | bool | `true` |  |
| deployment.replicas | int | `1` |  |
| env | object | `{}` |  |
| helmAddonsConfigMap | string | `"default-helm-addons-config"` |  |
| helmRepositoryImage.pullPolicy | string | `"IfNotPresent"` |  |
| helmRepositoryImage.repository | string | `"ghcr.io/nutanix-cloud-native/caren-helm-reg"` |  |
| helmRepositoryImage.tag | string | `""` |  |
| helmRepositorySecurityContext.fsGroup | int | `65534` |  |
| helmRepositorySecurityContext.runAsGroup | int | `65534` |  |
| helmRepositorySecurityContext.runAsUser | int | `65534` |  |
| hooks.ccm.aws.helmAddonStrategy.defaultValueTemplateConfigMap.create | bool | `true` |  |
| hooks.ccm.aws.helmAddonStrategy.defaultValueTemplateConfigMap.name | string | `"default-aws-ccm-helm-values-template"` |  |
| hooks.ccm.aws.k8sMinorVersionToCCMVersion."1.27" | string | `"v1.27.9"` |  |
| hooks.ccm.aws.k8sMinorVersionToCCMVersion."1.28" | string | `"v1.28.9"` |  |
| hooks.ccm.aws.k8sMinorVersionToCCMVersion."1.29" | string | `"v1.29.6"` |  |
| hooks.ccm.aws.k8sMinorVersionToCCMVersion."1.30" | string | `"v1.30.3"` |  |
| hooks.ccm.aws.k8sMinorVersionToCCMVersion."1.31" | string | `"v1.31.0"` |  |
| hooks.ccm.nutanix.helmAddonStrategy.defaultValueTemplateConfigMap.create | bool | `true` |  |
| hooks.ccm.nutanix.helmAddonStrategy.defaultValueTemplateConfigMap.name | string | `"default-nutanix-ccm-helm-values-template"` |  |
| hooks.clusterAutoscaler.crsStrategy.defaultInstallationConfigMap.name | string | `"cluster-autoscaler"` |  |
| hooks.clusterAutoscaler.helmAddonStrategy.defaultValueTemplateConfigMap.create | bool | `true` |  |
| hooks.clusterAutoscaler.helmAddonStrategy.defaultValueTemplateConfigMap.name | string | `"default-cluster-autoscaler-helm-values-template"` |  |
| hooks.cni.calico.crsStrategy.defaultInstallationConfigMaps.AWSCluster.configMap.content | string | `""` |  |
| hooks.cni.calico.crsStrategy.defaultInstallationConfigMaps.AWSCluster.configMap.name | string | `"calico-cni-crs-installation-awscluster"` |  |
| hooks.cni.calico.crsStrategy.defaultInstallationConfigMaps.AWSCluster.create | bool | `true` |  |
| hooks.cni.calico.crsStrategy.defaultInstallationConfigMaps.DockerCluster.configMap.content | string | `""` |  |
| hooks.cni.calico.crsStrategy.defaultInstallationConfigMaps.DockerCluster.configMap.name | string | `"calico-cni-crs-installation-dockercluster"` |  |
| hooks.cni.calico.crsStrategy.defaultInstallationConfigMaps.DockerCluster.create | bool | `true` |  |
| hooks.cni.calico.crsStrategy.defaultInstallationConfigMaps.NutanixCluster.configMap.content | string | `""` |  |
| hooks.cni.calico.crsStrategy.defaultInstallationConfigMaps.NutanixCluster.configMap.name | string | `"calico-cni-crs-installation-nutanixcluster"` |  |
| hooks.cni.calico.crsStrategy.defaultInstallationConfigMaps.NutanixCluster.create | bool | `true` |  |
| hooks.cni.calico.crsStrategy.defaultTigeraOperatorConfigMap.name | string | `"tigera-operator"` |  |
| hooks.cni.calico.defaultPodSubnet | string | `"192.168.0.0/16"` |  |
| hooks.cni.calico.helmAddonStrategy.defaultValueTemplatesConfigMaps.AWSCluster.create | bool | `true` |  |
| hooks.cni.calico.helmAddonStrategy.defaultValueTemplatesConfigMaps.AWSCluster.name | string | `"calico-cni-helm-values-template-awscluster"` |  |
| hooks.cni.calico.helmAddonStrategy.defaultValueTemplatesConfigMaps.DockerCluster.create | bool | `true` |  |
| hooks.cni.calico.helmAddonStrategy.defaultValueTemplatesConfigMaps.DockerCluster.name | string | `"calico-cni-helm-values-template-dockercluster"` |  |
| hooks.cni.calico.helmAddonStrategy.defaultValueTemplatesConfigMaps.NutanixCluster.create | bool | `true` |  |
| hooks.cni.calico.helmAddonStrategy.defaultValueTemplatesConfigMaps.NutanixCluster.name | string | `"calico-cni-helm-values-template-nutanixcluster"` |  |
| hooks.cni.cilium.crsStrategy.defaultCiliumConfigMap.name | string | `"cilium"` |  |
| hooks.cni.cilium.helmAddonStrategy.defaultValueTemplateConfigMap.create | bool | `true` |  |
| hooks.cni.cilium.helmAddonStrategy.defaultValueTemplateConfigMap.name | string | `"default-cilium-cni-helm-values-template"` |  |
| hooks.csi.aws-ebs.helmAddonStrategy.defaultValueTemplateConfigMap.create | bool | `true` |  |
| hooks.csi.aws-ebs.helmAddonStrategy.defaultValueTemplateConfigMap.name | string | `"default-aws-ebs-csi-helm-values-template"` |  |
| hooks.csi.local-path.helmAddonStrategy.defaultValueTemplateConfigMap.create | bool | `true` |  |
| hooks.csi.local-path.helmAddonStrategy.defaultValueTemplateConfigMap.name | string | `"default-local-path-provisioner-csi-helm-values-template"` |  |
| hooks.csi.nutanix.helmAddonStrategy.defaultValueTemplateConfigMap.create | bool | `true` |  |
| hooks.csi.nutanix.helmAddonStrategy.defaultValueTemplateConfigMap.name | string | `"default-nutanix-csi-helm-values-template"` |  |
| hooks.csi.snapshot-controller.helmAddonStrategy.defaultValueTemplateConfigMap.create | bool | `true` |  |
| hooks.csi.snapshot-controller.helmAddonStrategy.defaultValueTemplateConfigMap.name | string | `"default-snapshot-controller-helm-values-template"` |  |
| hooks.nfd.crsStrategy.defaultInstallationConfigMap.name | string | `"node-feature-discovery"` |  |
| hooks.nfd.helmAddonStrategy.defaultValueTemplateConfigMap.create | bool | `true` |  |
| hooks.nfd.helmAddonStrategy.defaultValueTemplateConfigMap.name | string | `"default-nfd-helm-values-template"` |  |
| hooks.serviceLoadBalancer.metalLB.defaultValueTemplateConfigMap.create | bool | `true` |  |
| hooks.serviceLoadBalancer.metalLB.defaultValueTemplateConfigMap.name | string | `"default-metallb-helm-values-template"` |  |
| hooks.virtualIP.kubeVip.defaultTemplateConfigMap.create | bool | `true` |  |
| hooks.virtualIP.kubeVip.defaultTemplateConfigMap.name | string | `"default-kube-vip-template"` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix"` |  |
| image.tag | string | `""` |  |
| imagePullSecrets | list | `[]` | Optional secrets used for pulling the container image |
| namespaceSync.enabled | bool | `true` |  |
| namespaceSync.sourceNamespace | string | `""` |  |
| namespaceSync.targetNamespaceLabelKey | string | `"caren.nutanix.com/namespace-sync"` |  |
| nodeSelector | object | `{}` |  |
| priorityClassName | string | `"system-cluster-critical"` | Priority class to be used for the pod. |
| resources.limits.cpu | string | `"100m"` |  |
| resources.limits.memory | string | `"256Mi"` |  |
| resources.requests.cpu | string | `"100m"` |  |
| resources.requests.memory | string | `"128Mi"` |  |
| securityContext.runAsUser | int | `65532` |  |
| selfHostedRegistry | bool | `true` |  |
| service.annotations | object | `{}` |  |
| service.port | int | `443` |  |
| service.type | string | `"ClusterIP"` |  |
| tolerations | list | `[{"effect":"NoSchedule","key":"node-role.kubernetes.io/control-plane","operator":"Equal"}]` | Kubernetes pod tolerations |
