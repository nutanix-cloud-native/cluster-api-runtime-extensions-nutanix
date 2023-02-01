# syntax=docker/dockerfile:1.4

# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

FROM --platform=linux/amd64 gcr.io/distroless/static@sha256:93bb1b564033909a660111671303f9683e13f0567de95e4b6fde3226e532955e as linux-amd64
FROM --platform=linux/arm64 gcr.io/distroless/static@sha256:72fec9960c247e7e68ed1db4b5b561f6b6da437215fb41d5fd7df790ec6df1a7 as linux-arm64

FROM --platform=linux/${TARGETARCH} linux-${TARGETARCH}

COPY capi-runtime-extensions /usr/local/bin/capi-runtime-extensions

# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532
ENTRYPOINT ["/usr/local/bin/capi-runtime-extensions"]
