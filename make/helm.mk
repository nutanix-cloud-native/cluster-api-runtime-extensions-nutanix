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

# helmAddonsOverrides has a hand-maintained schema (additionalProperties, descriptions) because
# "helm schema" infers from values.yaml and only sees "helmAddonsOverrides: {}", so it emits
# just "type": "object" and drops the rest. We merge it back after generation.
HELM_ADDONS_OVERRIDES_SCHEMA := charts/cluster-api-runtime-extensions-nutanix/values.schema.helmAddonsOverrides.json
VALUES_SCHEMA := charts/cluster-api-runtime-extensions-nutanix/values.schema.json

.PHONY: schema-chart
schema-chart: ## Updates helm values JSON schema
schema-chart:
	helm schema \
	  --use-helm-docs \
	  --values charts/cluster-api-runtime-extensions-nutanix/values.yaml \
	  --output $(VALUES_SCHEMA)
	gojq --slurpfile addonsSchema $(HELM_ADDONS_OVERRIDES_SCHEMA) \
	  '.properties.helmAddonsOverrides = $$addonsSchema[0]' $(VALUES_SCHEMA) > $(VALUES_SCHEMA).tmp && mv $(VALUES_SCHEMA).tmp $(VALUES_SCHEMA)
