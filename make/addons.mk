# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# Addon metadata (Chart name, Chart repo, Chart version, App version, Repo, Release) above each version.
# Source repos are assumed fixed. To refresh release URLs when versions change, use a prompt like:
#
#   For each addon in addons.mk, update the chart version and app version from the comment block to the latest version
#   available from the Helm chart repo.
#   Update the metadata for each addon (Chart name, Chart repo, App version, Repo, Release) to the latest version.
#   Verify that the metadata is consistent with the corresponding addon kustomization in hack/addons/kustomize.
#   Run 'make addons.sync' to update the addons.

# Calico (tigera-operator)
#   Chart name: tigera-operator
#   Chart repo: https://docs.tigera.io/calico/charts/index.yaml
#   Chart version:   3.31.7
#   App version:     3.31.7
#   Repo:            https://github.com/projectcalico/calico
#   Release:         https://github.com/projectcalico/calico/releases/tag/v3.32.2
# Stay on 3.31.x: Calico v3.32+ unbundles CRDs from the tigera-operator chart and requires a separate
# crd.projectcalico.org.v1 chart (not yet wired into update-calico-manifests.sh).
export CALICO_VERSION := v3.32.2

# Cilium
#   Chart name: cilium
#   Chart repo: https://helm.cilium.io/index.yaml
#   Chart version:   1.20.2
#   App version:     1.20.2
#   Repo:            https://github.com/cilium/cilium
#   Release:         https://github.com/cilium/cilium/releases/tag/v1.20.2
export CILIUM_VERSION := 1.20.2

# Node Feature Discovery
#   Chart name: node-feature-discovery
#   Chart repo: https://kubernetes-sigs.github.io/node-feature-discovery/charts/index.yaml
#   Chart version:   0.19.0
#   App version:     0.19.0
#   Repo:            https://github.com/kubernetes-sigs/node-feature-discovery
#   Release:         https://github.com/kubernetes-sigs/node-feature-discovery/releases/tag/v0.19.0
export NODE_FEATURE_DISCOVERY_VERSION := 0.19.0

# Cluster Autoscaler
#   Chart name: cluster-autoscaler
#   Chart repo: https://kubernetes.github.io/autoscaler/index.yaml
#   Chart version:   9.59.0
#   App version:     1.35.0
#   Repo:            https://github.com/kubernetes/autoscaler
#   Release:         https://github.com/kubernetes/autoscaler/releases/tag/cluster-autoscaler-1.35.0
# TODO: Remove tag override once https://github.com/kubernetes/autoscaler/issues/9439 is resolved.
export CLUSTER_AUTOSCALER_CHART_VERSION := 9.59.0

# AWS EBS CSI
#   Chart name: aws-ebs-csi-driver
#   Chart repo: https://kubernetes-sigs.github.io/aws-ebs-csi-driver/index.yaml
#   Chart version:   2.66.0
#   App version:     1.66.0
#   Repo:            https://github.com/kubernetes-sigs/aws-ebs-csi-driver
#   Release:         https://github.com/kubernetes-sigs/aws-ebs-csi-driver/releases/tag/v1.66.0
export AWS_EBS_CSI_CHART_VERSION := 2.66.0

# Nutanix Storage CSI
#   Chart name: nutanix-csi-storage
#   Chart repo: https://nutanix.github.io/helm-releases/index.yaml
#   Chart version:   3.8.0
#   App version:     3.8.0
#   Repo:            https://portal.nutanix.com/page/documents/list?type=software&filterKey=product&filterVal=CSI
#   Release:         https://portal.nutanix.com/page/documents/details?targetId=CSI-Volume-Driver-v3_8:CSI-Volume-Driver-v3_8
export NUTANIX_STORAGE_CSI_CHART_VERSION := 3.8.0

# Local Path Provisioner CSI
#   Chart name: local-path-provisioner
#   Chart repo: https://charts.containeroo.ch/index.yaml
#   Chart version:   0.0.38
#   App version:     v0.0.37
#   Repo:            https://github.com/rancher/local-path-provisioner
#   Release:         https://github.com/rancher/local-path-provisioner/releases/tag/v0.0.37
export LOCAL_PATH_CSI_CHART_VERSION := 0.0.38

# Snapshot Controller
#   Chart name: snapshot-controller
#   Chart repo: https://piraeus.io/helm-charts/index.yaml
#   Chart version:   5.3.0
#   App version:     8.6.0
#   Repo:            https://github.com/kubernetes-csi/external-snapshotter
#   Release:         https://github.com/kubernetes-csi/external-snapshotter/releases/tag/v8.6.0
export SNAPSHOT_CONTROLLER_CHART_VERSION := 5.3.0

# AWS CCM (chart version same for all K8s; app image version per K8s minor - check image exists via crane ls registry.k8s.io/provider-aws/cloud-controller-manager)
#   Chart name: aws-cloud-controller-manager
#   Chart repo: https://kubernetes.github.io/cloud-provider-aws/index.yaml
#   Chart version:   0.0.12
#   App versions:     per K8s minor (see AWS_CCM_VERSION_* below)
#   Repo:            https://github.com/kubernetes/cloud-provider-aws
#   Releases:        v1.33.4 https://github.com/kubernetes/cloud-provider-aws/releases/tag/v1.33.4
#                    v1.34.3 https://github.com/kubernetes/cloud-provider-aws/releases/tag/v1.34.3
#                    v1.35.2 https://github.com/kubernetes/cloud-provider-aws/releases/tag/v1.35.2
#                    v1.36.1 https://github.com/kubernetes/cloud-provider-aws/releases/tag/v1.36.1
#                    v1.37.0 https://github.com/kubernetes/cloud-provider-aws/releases/tag/v1.37.0
# AWS CCM uses the same chart version for all kubernetes versions. The image used in the deployment will
# be updated by the addon kustomization for CRS deployments and via Helm values for HelmAddon deployments.
export AWS_CCM_CHART_VERSION := 0.0.12
# A map of AWS CCM versions.
export AWS_CCM_VERSION_133 := v1.33.4
export AWS_CCM_VERSION_134 := v1.34.3
export AWS_CCM_VERSION_135 := v1.35.2
export AWS_CCM_VERSION_136 := v1.36.1
export AWS_CCM_VERSION_137 := v1.37.0

# AWS Load Balancer Controller
#   Chart name: aws-load-balancer-controller
#   Chart repo: https://aws.github.io/eks-charts/index.yaml
#   Chart version:   3.5.0
#   App version:     v3.5.0
#   Repo:            https://github.com/kubernetes-sigs/aws-load-balancer-controller (app; chart from aws/eks-charts)
#   Release:         https://github.com/kubernetes-sigs/aws-load-balancer-controller/releases/tag/v3.5.0
export AWS_LOAD_BALANCER_CONTROLLER_CHART_VERSION := 3.5.0

# Nutanix CCM
#   Chart name:    nutanix-cloud-provider
#   Chart repo:    https://nutanix.github.io/helm/index.yaml
#   Chart version: 0.6.2
#   App version:   v0.6.1
#   Repo:          https://github.com/nutanix-cloud-native/cloud-provider-nutanix
#   Release:       https://github.com/nutanix-cloud-native/cloud-provider-nutanix/releases/tag/v0.6.1
# Latest app releases (v0.6.3 / v0.7.0) publish Helm charts to OCI only; classic helm index still tops at 0.6.2.
export NUTANIX_CCM_CHART_VERSION := 0.6.2

# MetalLB
#   Chart name:    metallb
#   Chart repo:    https://metallb.github.io/metallb/index.yaml
#   Chart version: 0.16.1
#   App version:   v0.16.1
#   Repo:          https://github.com/metallb/metallb
#   Release:       https://github.com/metallb/metallb/releases/tag/v0.16.1
export METALLB_CHART_VERSION := 0.16.1

# COSI Controller
#   Chart name:    cosi-controller
#   Chart repo: 	 https://mesosphere.github.io/charts/stable/index.yaml
#   Chart version: 0.2.2
#   App version:   v0.2.2
#   Repo:          https://github.com/kubernetes-sigs/container-object-storage-interface
#   Release:       https://github.com/kubernetes-sigs/container-object-storage-interface/releases/tag/v0.2.2
export COSI_CONTROLLER_VERSION := 0.2.2

# Konnector Agent
#   Chart name:    konnector-agent
#   Chart repo: 	 https://nutanix.github.io/helm-releases/index.yaml
#   Chart version: 1.4.2
#   App version:   v1.4.2
#   Repo:          https://github.com/nutanix-core/k8s-agent
#   Release:       https://github.com/nutanix-core/k8s-agent/releases/tag/1.4.2
# Stay on 1.4.2: 1.5.0 caused Nutanix e2e cluster deletion to hang on BeforeClusterDelete
# (Konnector HelmChartProxy uninstall / PC deregistration never completed within 30m).
export KONNECTOR_AGENT_VERSION := 1.4.2

# Multus
#   Chart name: multus
#   Chart repo: https://mesosphere.github.io/charts/stable/index.yaml
#   Chart version:   0.1.1
#   App version:     4.2.4
#   Repo:            https://github.com/k8snetworkplumbingwg/multus-cni
#   Release:         https://github.com/k8snetworkplumbingwg/multus-cni/releases/tag/v4.2.4
export MULTUS_CHART_VERSION := 0.1.1

# Nutanix Flow CNI
#   Chart name: nutanix-flow-cni
#   Chart repo: https://nutanix.github.io/helm-releases/index.yaml
#   Chart version:   1.1.0
#   App version:     1.1.0
#   Repo:            https://github.com/nutanix-core/flow-k8s-cni
#   Release:         https://github.com/nutanix-core/flow-k8s-cni/releases/tag/1.1.0
export NUTANIX_FLOW_CNI_VERSION := 1.1.0

# Kube-vip (container image, not Helm - latest version can be checked via crane ls ghcr.io/kube-vip/kube-vip)
#   Repo:        https://github.com/kube-vip/kube-vip
#   App version: v1.2.4
#   Release:     https://github.com/kube-vip/kube-vip/releases/tag/v1.2.4
export KUBE_VIP_VERSION := v1.2.4

.PHONY: addons.sync
addons.sync: $(addprefix update-addon.,calico cilium nfd cluster-autoscaler snapshot-controller local-path-provisioner-csi aws-ebs-csi kube-vip)
addons.sync: $(addprefix update-addon.aws-ccm.,133 134 135 136 137)
addons.sync: template-helm-repository

.PHONY: update-addon.calico
update-addon.calico: ; $(info $(M) updating calico manifests)
	./hack/addons/update-calico-manifests.sh

.PHONY: update-addon.cilium
update-addon.cilium: ; $(info $(M) updating cilium manifests)
	./hack/addons/update-cilium-manifests.sh

.PHONY: update-addon.nfd
update-addon.nfd: ; $(info $(M) updating node feature discovery manifests)
	./hack/addons/update-node-feature-discovery-manifests.sh

.PHONY: update-addon.cluster-autoscaler
update-addon.cluster-autoscaler: ; $(info $(M) updating cluster-autoscaler manifests)
	./hack/addons/update-cluster-autoscaler.sh

.PHONY: update-addon.aws-ebs-csi
update-addon.aws-ebs-csi: ; $(info $(M) updating aws ebs csi manifests)
	./hack/addons/update-aws-ebs-csi.sh

.PHONY: update-addon.local-path-provisioner-csi
update-addon.local-path-provisioner-csi: ; $(info $(M) updating local-path-provisioner csi manifests)
	./hack/addons/update-local-path-provisioner-csi.sh

.PHONY: update-addon.snapshot-controller
update-addon.snapshot-controller: ; $(info $(M) updating snapshot-controller manifests)
	./hack/addons/update-snapshot-controller.sh

.PHONY: update-addon.aws-ccm.%
update-addon.aws-ccm.%: ; $(info $(M) updating aws ccm $* manifests)
	./hack/addons/update-aws-ccm.sh $(AWS_CCM_VERSION_$*) $(AWS_CCM_CHART_VERSION)

.PHONY: update-addon.kube-vip
update-addon.kube-vip: ; $(info $(M) updating kube-vip manifests)
	./hack/addons/update-kube-vip-manifests.sh

.PHONY: generate-helm-configmap
generate-helm-configmap:
	go run hack/tools/helm-cm/main.go -kustomize-directory="./hack/addons/kustomize" \
	  -output-file="./charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml"
	./hack/addons/add-warning-helm-configmap.sh

# Set only the supported CSI providers for each provider.
.PHONY: configure-csi-providers
configure-csi-providers: ; $(info $(M) configuring supported csi providers)
	./hack/addons/configure-supported-csi-providers.sh

.PHONY: generate-mindthegap-repofile
generate-mindthegap-repofile: generate-helm-configmap ; $(info $(M) generating helm repofile for mindthgap)
	./hack/addons/generate-mindthegap-repofile.sh

.PHONY: template-helm-repository
template-helm-repository: generate-mindthegap-repofile ## this is used by gorealeaser to set the helm value to this.
	yq -i '.data |= (to_entries | map(.value |= (. | fromjson | .RepositoryURL |= "{{ if .Values.helmRepository.enabled }}oci://helm-repository.{{ .Release.Namespace }}.svc/charts{{ else }}" + . + "{{ end }}" | to_yaml)) | from_entries)' ./charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml

.PHONY: list-images
list-images:
	cd hack/tools/fetch-images && go run . \
	  -chart-directory=$(PWD)/charts/cluster-api-runtime-extensions-nutanix/ \
	  -helm-chart-configmap=$(PWD)/charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml \
	  -caren-version=$(CAREN_VERSION) \
	  -additional-yaml-files=$(PWD)/charts/cluster-api-runtime-extensions-nutanix/defaultclusterclasses/nutanix-cluster-class.yaml
