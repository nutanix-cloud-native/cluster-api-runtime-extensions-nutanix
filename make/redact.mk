# Copyright 2024 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

ARTIFACT_DIR := $(REPO_ROOT)/_artifacts

# Define the placeholder value
REDACTED_PLACEHOLDER="***REDACTED***"

.PHONY: redact-artifacts
redact-artifacts: ## Redacts all secrets in $(ARTIFACT_DIR)
	go run hack/tools/redact-artifacts/main.go
