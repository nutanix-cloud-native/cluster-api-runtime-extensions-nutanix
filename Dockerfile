# syntax=docker/dockerfile:1.4

# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

FROM --platform=linux/amd64 gcr.io/distroless/static@sha256:b1a9ecc1b37ec3bc9a2d8f8a0a5090ee5babda422f2b1c3724f108fa7e506e77 as linux-amd64
FROM --platform=linux/arm64 gcr.io/distroless/static@sha256:2fc464bd828a7f713020c06f0bb4ff01670d08292ba4a01ffbc3b56a1d59da3a as linux-arm64

FROM --platform=linux/${TARGETARCH} linux-${TARGETARCH}

COPY capi-runtime-extensions /usr/local/bin/capi-runtime-extensions

# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532
ENTRYPOINT ["/usr/local/bin/capi-runtime-extensions"]
