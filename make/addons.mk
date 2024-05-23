# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CALICO_VERSION := v3.28.0
export CILIUM_VERSION := 1.15.5
export NODE_FEATURE_DISCOVERY_VERSION := 0.15.2
export CLUSTER_AUTOSCALER_VERSION := 9.37.0
export AWS_CSI_SNAPSHOT_CONTROLLER_VERSION := v6.3.3
export AWS_EBS_CSI_CHART_VERSION := v2.28.1
export NUTANIX_STORAGE_CSI_CHART_VERSION := v3.0.0-beta.1912
export NUTANIX_SNAPSHOT_CSI_CHART_VERSION := v6.3.2
# a map of AWS CCM versions
export AWS_CCM_VERSION_127 := v1.27.1
export AWS_CCM_CHART_VERSION_127 := 0.0.8
export AWS_CCM_VERSION_128 := v1.28.1
export AWS_CCM_CHART_VERSION_128 := 0.0.8
export AWS_CCM_VERSION_129 := v1.29.2
export AWS_CCM_CHART_VERSION_129 := 0.0.8

export NUTANIX_CCM_CHART_VERSION := 0.3.3

export KUBE_VIP_VERSION := v0.8.0

export METALLB_CHART_VERSION := v0.14.5

.PHONY: addons.sync
addons.sync: $(addprefix update-addon.,calico cilium nfd cluster-autoscaler aws-ebs-csi aws-ccm.127 aws-ccm.128 aws-ccm.129 kube-vip)

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

.PHONY: update-addon.aws-ccm.%
update-addon.aws-ccm.%: ; $(info $(M) updating aws ccm $* manifests)
	./hack/addons/update-aws-ccm.sh $(AWS_CCM_VERSION_$*) $(AWS_CCM_CHART_VERSION_$*)

.PHONY: update-addon.kube-vip
update-addon.kube-vip: ; $(info $(M) updating kube-vip manifests)
	./hack/addons/update-kube-vip-manifests.sh

.PHONY: generate-helm-configmap
generate-helm-configmap:
	go run hack/tools/helm-cm/main.go -kustomize-directory="./hack/addons/kustomize" -output-file="./charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml"
	./hack/addons/add-warning-helm-configmap.sh
