# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CAPA_REPO := https://github.com/kubernetes-sigs/cluster-api-provider-aws.git
export CAPA_VERSION := v2.2.2
export CAPA_API_PACKAGE_NAME := sigs.k8s.io/cluster-api-provider-aws/v2/api/

# It is not possible to resolved Kubernetes and controller-runtime modules for the different infrastructure providers.
# Instead. sync their APIs into the common/pkg/external directory.
.PHONY: apis.sync
apis.sync: ## Syncs infrastructure providers' APIs
	$(MAKE) api.cluster-api-provider-aws.sync \
		PROVIDER_VERSION=$(CAPA_VERSION) PROVIDER_REPO=$(CAPA_REPO) PROVIDER_API_PACKAGE_NAME=$(CAPA_API_PACKAGE_NAME)
	# Run go generate to fix minor formatting issues.
	$(MAKE) go-generate
	$(MAKE) mod-tidy.common

.PHONY: %
api.%.sync: ## Syncs an infrastructure provider's API
api.%.sync: CLONE_DIR=$(REPO_ROOT)/.local/infrastructure-providers/$*
api.%.sync: API_DIR=common/pkg/external/$*
api.%.sync: NEW_PACKAGE_NAME=github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/$*/api/
api.%.sync:
	rm -rf $(CLONE_DIR) && mkdir -p $(CLONE_DIR)
	git clone -c advice.detachedHead=false --depth=1 --branch=$(PROVIDER_VERSION) $(PROVIDER_REPO) $(CLONE_DIR)
	rm -rf $(API_DIR) && mkdir -p $(API_DIR)
	rsync -av \
		--exclude='api/*/*webhook*.go' \
		--exclude='api/*/*test.go'     \
		--exclude='api/*/s3bucket.go'  \
		$(CLONE_DIR)/api \
		$(API_DIR)
	rm -rf $(CLONE_DIR)
	find . -type f -name "*.go" -exec sed -i -- \
		's|$(PROVIDER_API_PACKAGE_NAME)|$(NEW_PACKAGE_NAME)|g' {} +
