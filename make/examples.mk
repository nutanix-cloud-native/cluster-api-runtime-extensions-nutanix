# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: examples.sync
examples.sync: ## Syncs the examples by fetching upstream examples and applying kustomize patches
examples.sync: update-addon.kube-vip # kube-vip is part of the KCP spec
examples.sync: ; $(info $(M) syncing examples)
	hack/examples/sync.sh
