# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: examples.sync examples.sync.nutanix-auth examples.sync.nutanix-overlay
examples.sync: ## Syncs examples with internal/main-compatible Nutanix output (basic_auth)
examples.sync: ; $(info $(M) syncing examples)
	$(MAKE) examples.sync.nutanix-overlay

examples.sync.nutanix-auth: ## Syncs examples with Nutanix auth overlays (env-driven: api_key preferred when present)
examples.sync.nutanix-auth: ; $(info $(M) syncing examples with nutanix-auth overlays)
	NUTANIX_USE_AUTH_OVERLAYS=true $(MAKE) examples.sync.nutanix-overlay

examples.sync.nutanix-overlay: ## Syncs examples applying Nutanix credentials overlay selection
examples.sync.nutanix-overlay: update-addon.kube-vip # kube-vip is part of the KCP spec
	hack/examples/sync.sh
