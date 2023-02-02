# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CALICO_VERSION := v3.24

.PHONY: update-addon.calico
update-addon.calico: install-tool.gojq install-tool.kubectl ; $(info $(M) updating calico manifests)
	./hack/addons/update-calico-manifests.sh
