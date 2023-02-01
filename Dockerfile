# syntax=docker/dockerfile:1.4

# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# Production image
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY /bin/manager .
# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532
ENTRYPOINT ["/manager"]
