# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: chart-docs
chart-docs: ## Update helm chart docs
chart-docs:
	helm-docs --chart-search-root=charts

.PHONY: lint-chart
lint-chart: ## Lints helm chart and addon values templates
lint-chart:
	ct lint --all --config charts/ct-config.yaml
	# The default ClusterClass templates (templates/clusterclass_{nutanix,aws,eks,docker}.yaml)
	# guard their output behind `.Capabilities.APIVersions.Has "<group>/<version>/<Kind>"` so that
	# the chart only installs a provider's default ClusterClass when that infrastructure provider's
	# CRD is present in the management cluster. Because `helm template` renders offline without a
	# live API server, `.Capabilities.APIVersions` only contains built-in Kubernetes APIs by default
	# and those templates would render as empty. Passing `--api-versions` for each provider's
	# infrastructure template Kind populates `.Capabilities.APIVersions` so that all default
	# ClusterClass templates are rendered and included in the linted output.
	#
	# `ct lint` only runs `yamllint` on `Chart.yaml` and `values.yaml` (not on `templates/` or
	# rendered output), and `helm lint` only checks basic YAML parseability of rendered templates.
	# Piping `helm template` output into `yamllint --strict -c charts/lintconf.yaml -` enforces
	# strict YAML formatting and syntax rules (such as block scalar headers, indentation, comment
	# spacing, and trailing whitespace) across all rendered chart manifests.
	helm template charts/cluster-api-runtime-extensions-nutanix \
	  --api-versions=infrastructure.cluster.x-k8s.io/v1beta1/NutanixClusterTemplate \
	  --api-versions=infrastructure.cluster.x-k8s.io/v1beta2/AWSClusterTemplate \
	  --api-versions=infrastructure.cluster.x-k8s.io/v1beta2/AWSManagedClusterTemplate \
	  --api-versions=infrastructure.cluster.x-k8s.io/v1beta1/DockerClusterTemplate | \
	  yamllint --strict -c charts/lintconf.yaml -
	# Files under `charts/cluster-api-runtime-extensions-nutanix/addons/` (e.g. `values-template.yaml`)
	# are not rendered as Helm chart templates; instead, the chart embeds them verbatim into
	# ConfigMaps via `.Files.Get`, and CAREN lifecycle handlers or CAAPH render them at runtime as
	# Go templates using `Cluster` / addon input structs. Running `TestAddonValuesTemplates` renders
	# every file under `addons/` across its conditional branches and validates the resulting YAML
	# with `sigs.k8s.io/yaml` and `yamllint --strict -c charts/lintconf.yaml`.
	go test -count=1 ./pkg/handlers/lifecycle/addons -run '^TestAddonValuesTemplates$$'

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
	  --values charts/cluster-api-runtime-extensions-nutanix/values.yaml \
	  --output charts/cluster-api-runtime-extensions-nutanix/values.schema.json
