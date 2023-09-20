# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CALICO_VERSION := v3.26.1
export NODE_FEATURE_VERSION := 0.14.1

.PHONY: update-addon.calico
update-addon.calico: ; $(info $(M) updating calico manifests)
	./hack/addons/update-calico-manifests.sh

.PHONY: update-addon.nfd
update-addon.nfd: ; $(info $(M) updating node feature discovery manifests)
	./hack/addons/update-node-feature-discovery-manifests.sh
