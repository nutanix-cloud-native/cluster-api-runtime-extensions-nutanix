# Copyright 2024 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: preview-docs
preview-docs: ## Runs hugo server to locally preview docs
preview-docs: export HUGO_PARAMS_defaultKubernetesVersion ?= $(E2E_DEFAULT_KUBERNETES_VERSION)
preview-docs: ; $(info $(M) running hugo server to locally preview docs)
	cd docs && hugo server --buildFuture --buildDrafts

.PHONY: build-docs
build-docs: ## Builds the docs
build-docs: export HUGO_PARAMS_defaultKubernetesVersion ?= $(E2E_DEFAULT_KUBERNETES_VERSION)
build-docs: ; $(info $(M) building docs with hugo)
ifndef BASE_URL
	$(error BASE_URL is not set)
endif
	cd docs && hugo --minify --baseURL "$(BASE_URL)"
