# Copyright 2026 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: handlers.check-frozen
handlers.check-frozen: ## Checks that released golden output, handler code and handler names are unchanged
handlers.check-frozen: ; $(info $(M) checking released handlers are unchanged)
	hack/handlers/check-frozen-selftest.sh
	hack/handlers/check-frozen.sh

.PHONY: handlers.new-version
handlers.new-version: ## Cuts a new handler version from the latest one once that is released
handlers.new-version: ; $(info $(M) cutting a new handler version)
	hack/handlers/new-version.sh
	$(MAKE) examples.sync handlers.golden-update

.PHONY: handlers.golden-update
handlers.golden-update: ## Regenerates golden output of the open, unreleased handler version
# TODO: Remove once https://github.com/golang/go/issues/75031 is fixed.
handlers.golden-update: export GOTOOLCHAIN := $(shell go version | cut -d ' ' -f3)+auto
handlers.golden-update: ; $(info $(M) updating golden output of the open handler version)
	open_version="$$(hack/handlers/open-version.sh)" && \
	source <(setup-envtest use -p env $(ENVTEST_VERSION)) && \
	go test ./pkg/handlers/golden/ -run '^TestGolden$$' -count=1 -update-golden="$${open_version}"
