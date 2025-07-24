# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: chart-docs
chart-docs: ## Update helm chart docs
chart-docs:
	helm-docs --chart-search-root=charts

.PHONY: lint-chart
lint-chart: ## Lints helm chart
lint-chart:
	ct lint --config charts/ct-config.yaml

.PHONY: lint-and-install-chart
lint-and-install-chart: ## Lints and installs helm chart
lint-and-install-chart:
	ct lint-and-install --config charts/ct-config.yaml
	ct lint-and-install --config charts/ct-config.yaml --upgrade

.PHONY: schema-chart
schema-chart: ## Updates helm values JSON schema
schema-chart:
	helm schema \
	  --use-helm-docs \
	  --input charts/cluster-api-runtime-extensions-nutanix/values.yaml \
	  --output charts/cluster-api-runtime-extensions-nutanix/values.schema.json
