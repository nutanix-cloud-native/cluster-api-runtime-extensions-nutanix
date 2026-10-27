# Copyright 2026 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: handlers.golden-update
handlers.golden-update: ## Regenerates golden output of the open, unreleased handler version
# TODO: Remove once https://github.com/golang/go/issues/75031 is fixed.
handlers.golden-update: export GOTOOLCHAIN := $(shell go version | cut -d ' ' -f3)+auto
handlers.golden-update: ; $(info $(M) updating golden output of the open handler version)
	open_version="$$(hack/handlers/open-version.sh)" && \
	source <(setup-envtest use -p env $(ENVTEST_VERSION)) && \
	go test ./pkg/handlers/golden/ -run '^TestGolden$$' -count=1 -update-golden="$${open_version}"
