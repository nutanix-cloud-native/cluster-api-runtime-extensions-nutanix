# syntax=docker/dockerfile:1.4

# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

FROM --platform=linux/amd64 gcr.io/distroless/static@sha256:39e460e64a929bb6d08a7b899eb76c78c17a487b84f7cfe5191415473423471f as linux-amd64
FROM --platform=linux/arm64 gcr.io/distroless/static@sha256:b5e90ec08ae3e1e72b28a92caf75e9e9f6eae54624e34486155349843d420126 as linux-arm64

FROM --platform=linux/${TARGETARCH} linux-${TARGETARCH}

COPY capi-runtime-extensions /usr/local/bin/capi-runtime-extensions

# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532
ENTRYPOINT ["/usr/local/bin/capi-runtime-extensions"]
