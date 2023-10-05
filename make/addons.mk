# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CALICO_VERSION := v3.26.1
export NODE_FEATURE_DISCOVERY_VERSION := 0.14.1
export AWS_CSI_SNAPSHOT_CONTROLLER_VERSION := v6.3.0
export AWS_EBS_CSI_VERSION := release-1.23

.PHONY: addons.sync
addons.sync: $(addprefix update-addon.,calico nfd aws-ebs-csi)

.PHONY: update-addon.calico
update-addon.calico: ; $(info $(M) updating calico manifests)
	./hack/addons/update-calico-manifests.sh

.PHONY: update-addon.nfd
update-addon.nfd: ; $(info $(M) updating node feature discovery manifests)
	./hack/addons/update-node-feature-discovery-manifests.sh

.PHONY: update-addon.aws-ebs-csi
update-addon.aws-ebs-csi: ; $(info $(M) updating aws ebs csi manifests)
	./hack/addons/update-aws-ebs-csi.sh
