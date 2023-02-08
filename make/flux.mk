# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: flux.install
flux.install: install-tool.flux2
	flux install --components=source-controller,helm-controller
