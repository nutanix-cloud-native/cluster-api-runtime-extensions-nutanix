# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

PROVIDER_MODULE_capa := sigs.k8s.io/cluster-api-provider-aws/v2
PROVIDER_API_PATH_capa := api
PROVIDER_API_VERSION_capa := v1beta2

PROVIDER_MODULE_capd := sigs.k8s.io/cluster-api/test
PROVIDER_API_PATH_capd := infrastructure/docker/api
PROVIDER_API_VERSION_capd := v1beta1

PROVIDER_MODULE_caaph := sigs.k8s.io/cluster-api-addon-provider-helm
PROVIDER_API_PATH_caaph := api
PROVIDER_API_VERSION_caaph := v1alpha1

PROVIDER_MODULE_capx := github.com/nutanix-cloud-native/cluster-api-provider-nutanix
PROVIDER_API_PATH_capx := api
PROVIDER_API_VERSION_capx := v1beta1

PROVIDER_MODULE_cabpk := sigs.k8s.io/cluster-api
PROVIDER_API_PATH_cabpk := bootstrap/kubeadm/api
PROVIDER_API_VERSION_cabpk := v1beta1

# It is not possible to resolve Kubernetes and controller-runtime modules for the different infrastructure providers
# without hitting dependency conflicts.
# Instead. sync their APIs into the api/external directory.
.PHONY: apis.sync
apis.sync: ## Syncs infrastructure providers' APIs
apis.sync: $(addprefix api.sync.,capa capd caaph capx cabpk) go-fix.api mod-tidy.api

.PHONY: api.sync.%
api.sync.%: ## Syncs an infrastructure provider's API
api.sync.%: PROVIDER_MODULE_DIR=$(REPO_ROOT)/hack/third-party/$*
api.sync.%: PROVIDER_API_DIR=api/external/$(PROVIDER_MODULE_$*)/$(PROVIDER_API_PATH_$*)/$(PROVIDER_API_VERSION_$*)/
api.sync.%: ; $(info $(M) syncing external API: $(PROVIDER_MODULE_$*)/$(PROVIDER_API_PATH_$*)/$(PROVIDER_API_VERSION_$*))
	cd $(PROVIDER_MODULE_DIR) && go mod tidy
	mkdir -p $(PROVIDER_API_DIR)
	rsync \
	  --recursive --delete --times --links --verbose --prune-empty-dirs \
	  --exclude='*webhook*.go' \
	  --exclude='*test.go'     \
	  --exclude='s3bucket.go'  \
	  $$(cd $(PROVIDER_MODULE_DIR) && go list -m -f '{{ .Dir }}' $(PROVIDER_MODULE_$*))/$(PROVIDER_API_PATH_$*)/$(PROVIDER_API_VERSION_$*)/*.go \
	  $(PROVIDER_API_DIR)
	find $(PROVIDER_API_DIR) -type d -exec chmod 0755 {} \;
	find $(PROVIDER_API_DIR) -type f -exec chmod 0644 {} \;
