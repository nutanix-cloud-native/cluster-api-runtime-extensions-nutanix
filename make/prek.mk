# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

ifneq ($(wildcard $(REPO_ROOT)/.pre-commit-config.yaml),)
	PREK_CONFIG_FILE ?= $(REPO_ROOT)/.pre-commit-config.yaml
else
	PREK_CONFIG_FILE ?= $(REPO_ROOT)/repo-infra/.pre-commit-config.yaml
endif

.PHONY: pre-commit prek
pre-commit: ## Runs pre-commit checks on all files
pre-commit: prek

prek: ## Runs prek on all files
prek: ; $(info $(M) running prek)
ifeq ($(wildcard $(PREK_CONFIG_FILE)),)
	$(error Cannot find prek config file $(PREK_CONFIG_FILE). Specify the config file via PREK_CONFIG_FILE variable)
endif
	env SKIP=$(SKIP) prek run -a --show-diff-on-failure --config $(PREK_CONFIG_FILE)
