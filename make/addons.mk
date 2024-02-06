# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CALICO_VERSION := $(shell goprintconst -file pkg/handlers/generic/lifecycle/cni/calico/strategy_helmaddon.go -name defaultCalicoHelmChartVersion)
export CILIUM_VERSION := $(shell goprintconst -file pkg/handlers/generic/lifecycle/cni/cilium/strategy_helmaddon.go -name defaultCiliumHelmChartVersion)
export NODE_FEATURE_DISCOVERY_VERSION := 0.14.1
export AWS_CSI_SNAPSHOT_CONTROLLER_VERSION := v6.3.0
export AWS_EBS_CSI_VERSION := v1.25.0
export AWS_CPI_VERSION_127 := v1.27.1

addons.sync: $(addprefix update-addon.,calico cilium nfd aws-ebs-csi)

.PHONY: update-addon.calico
update-addon.calico: ; $(info $(M) updating calico manifests)
	./hack/addons/update-calico-manifests.sh

.PHONY: update-addon.cilium
update-addon.cilium: ; $(info $(M) updating cilium manifests)
	./hack/addons/update-cilium-manifests.sh

.PHONY: update-addon.nfd
update-addon.nfd: ; $(info $(M) updating node feature discovery manifests)
	./hack/addons/update-node-feature-discovery-manifests.sh

.PHONY: update-addon.aws-ebs-csi
update-addon.aws-ebs-csi: ; $(info $(M) updating aws ebs csi manifests)
	./hack/addons/update-aws-ebs-csi.sh

.PHONY: update-addon.aws-cpi.127
update-addon.aws-cpi.127: ; $(info $(M) updating aws cpi manifests)
	./hack/addons/update-aws-cpi.sh $(AWS_CPI_VERSION_127)
