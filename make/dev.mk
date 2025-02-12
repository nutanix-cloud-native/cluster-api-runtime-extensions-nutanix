# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: dev.run-on-kind
dev.run-on-kind: export KUBECONFIG := $(KIND_KUBECONFIG)
dev.run-on-kind: kind.create clusterctl.init
ifndef SKIP_BUILD
dev.run-on-kind: release-snapshot
endif
dev.run-on-kind: SNAPSHOT_VERSION = $(shell gojq -r '.version+"-"+.runtime.goarch' dist/metadata.json)
dev.run-on-kind:
	kind load docker-image --name $(KIND_CLUSTER_NAME) \
		ko.local/cluster-api-runtime-extensions-nutanix:$(SNAPSHOT_VERSION) \
		ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-helm-chart-bundle-initializer:$(SNAPSHOT_VERSION)
	helm upgrade --install cluster-api-runtime-extensions-nutanix ./charts/cluster-api-runtime-extensions-nutanix \
		--set-string image.repository=ko.local/cluster-api-runtime-extensions-nutanix \
		--set-string image.tag=$(SNAPSHOT_VERSION) \
		--set-string helmRepository.images.bundleInitializer.tag=$(SNAPSHOT_VERSION) \
		--wait --wait-for-jobs
	kubectl rollout restart deployment cluster-api-runtime-extensions-nutanix
	kubectl rollout restart deployment helm-repository
	kubectl rollout status deployment cluster-api-runtime-extensions-nutanix
	kubectl rollout status deployment helm-repository

.PHONY: dev.update-webhook-image-on-kind
dev.update-webhook-image-on-kind: export KUBECONFIG := $(KIND_KUBECONFIG)
ifndef SKIP_BUILD
dev.update-webhook-image-on-kind: release-snapshot
endif
dev.update-webhook-image-on-kind: SNAPSHOT_VERSION = $(shell gojq -r '.version+"-"+.runtime.goarch' dist/metadata.json)
dev.update-webhook-image-on-kind:
	kind load docker-image --name $(KIND_CLUSTER_NAME) \
	  ko.local/cluster-api-runtime-extensions-nutanix:$(SNAPSHOT_VERSION)
	kubectl set image deployment \
	  cluster-api-runtime-extensions-nutanix manager=ko.local/cluster-api-runtime-extensions-nutanix:$(SNAPSHOT_VERSION)
	kubectl rollout restart deployment cluster-api-runtime-extensions-nutanix
	kubectl rollout status deployment cluster-api-runtime-extensions-nutanix

.PHONY: dev.update-bootstrap-credentials-aws
dev.update-bootstrap-credentials-aws: export KUBECONFIG := $(KIND_KUBECONFIG)
dev.update-bootstrap-credentials-aws:
	kubectl patch secret capa-manager-bootstrap-credentials -n capa-system -p="{\"data\":{\"credentials\": \"$$(clusterctl-aws bootstrap credentials encode-as-profile)\"}}"
	kubectl rollout restart deployment capa-controller-manager -n capa-system
	kubectl rollout status deployment capa-controller-manager -n capa-system

.PHONY: release-please
release-please:
# filter Returns all whitespace-separated words in text that do match any of the pattern words.
ifeq ($(filter main release/v%,$(GIT_CURRENT_BRANCH)),)
	$(error "release-please should only be run on the main or release branch")
else
	release-please release-pr --repo-url $(GITHUB_ORG)/$(GITHUB_REPOSITORY) --target-branch $(GIT_CURRENT_BRANCH) --token "$$(gh auth token)"
endif

.PHONY: .envrc.e2e
.envrc.e2e:
	gojq --yaml-input --raw-output '.variables | to_entries | map("export \(.key)=\(.value|tostring)")|.[]' < test/e2e/config/caren.yaml | envsubst > .envrc.e2e
	setup-envtest use -p env $(ENVTEST_VERSION) >> .envrc.e2e
	direnv reload
