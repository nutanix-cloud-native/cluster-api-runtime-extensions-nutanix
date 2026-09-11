# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

REPO_ROOT := $(CURDIR)

# Versions for tools that are not managed by devbox.
# The `!` suffix forces checking the remote API server
# for the latest patch version of the specified minor.
# Align with CAPI v1.13 (uses KUBEBUILDER_ENVTEST_KUBERNETES_VERSION=1.36.0)
# and this repo's k8s.io/* v0.35 dependencies.
ENVTEST_VERSION=1.35.x!

include make/all.mk
