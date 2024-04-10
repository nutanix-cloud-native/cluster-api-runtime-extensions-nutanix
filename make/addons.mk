# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CALICO_VERSION := v3.26.4
export CILIUM_VERSION := 1.15.0
export NODE_FEATURE_DISCOVERY_VERSION := 0.15.2
export CLUSTER_AUTOSCALER_VERSION := 9.35.0
export AWS_CSI_SNAPSHOT_CONTROLLER_VERSION := v6.3.3
export AWS_EBS_CSI_CHART_VERSION := v2.28.1
export NUTANIX_STORAGE_CSI_CHART_VERSION := v2.6.6
export NUTANIX_SNAPSHOT_CSI_CHART_VERSION := v6.3.2
# a map of AWS CCM versions
export AWS_CCM_VERSION_127 := v1.27.1
export AWS_CCM_CHART_VERSION_127 := 0.0.8
export AWS_CCM_VERSION_128 := v1.28.1
export AWS_CCM_CHART_VERSION_128 := 0.0.8

export NUTANIX_CCM_CHART_VERSION := 0.3.3

.PHONY: addons.sync
addons.sync: $(addprefix update-addon.,calico cilium nfd cluster-autoscaler aws-ebs-csi aws-ccm.127 nutanix-storage-csi aws-ccm.128)

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

.PHONY: update-addon.nutanix-storage-csi
update-addon.nutanix-storage-csi: ; $(info $(M) updating nutanix-storage csi manifests)
	./hack/addons/update-nutanix-csi.sh

.PHONY: generate-helm-configmap
generate-helm-configmap:
	go run hack/tools/helm-cm/main.go -kustomize-directory="./hack/addons/kustomize" -output-file="./charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml"
	./hack/addons/add-warning-helm-configmap.sh
