# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CAPA_API_PACKAGE_NAME := sigs.k8s.io/cluster-api-provider-aws/v2/api/
export CAPA_API_VERSION := v1beta2

# It is not possible to resolved Kubernetes and controller-runtime modules for the different infrastructure providers.
# Instead. sync their APIs into the common/pkg/external directory.
.PHONY: apis.sync
apis.sync: ## Syncs infrastructure providers' APIs
	$(MAKE) api.cluster-api-provider-aws_v2.sync \
		PROVIDER_API_PACKAGE_NAME=$(CAPA_API_PACKAGE_NAME) \
		PROVIDER_API_VERSION=$(CAPA_API_VERSION)
	# Run go generate to fix minor formatting issues.
	$(MAKE) go-generate
	$(MAKE) mod-tidy.common

.PHONY: %
api.%.sync: ## Syncs an infrastructure provider's API
api.%.sync: PROVIDER_MODULE_DIR=$(REPO_ROOT)/external/$(subst _,/,$*)
api.%.sync: PROVIDER_API_DIR=common/pkg/external/$(CAPA_API_PACKAGE_NAME)
api.%.sync: NEW_PACKAGE_NAME=github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/$(subst _,/,$*)/
api.%.sync:
	cd $(PROVIDER_MODULE_DIR) && go mod vendor
	rm -rf $(PROVIDER_API_DIR) && mkdir -p $(PROVIDER_API_DIR)
	rsync -av \
		--exclude='*webhook*.go' \
		--exclude='*test.go'     \
		--exclude='s3bucket.go'  \
		$(PROVIDER_MODULE_DIR)/vendor/$(PROVIDER_API_PACKAGE_NAME)/$(PROVIDER_API_VERSION) \
		$(PROVIDER_API_DIR)/
	find external/ -type f -name "*.go" -exec sed -i -- \
		's|$(PROVIDER_API_PACKAGE_NAME)|$(NEW_PACKAGE_NAME)|g' {} +
