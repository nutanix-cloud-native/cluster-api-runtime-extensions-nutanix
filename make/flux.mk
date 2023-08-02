# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: flux.install
flux.install:
	flux install --components=source-controller,helm-controller
