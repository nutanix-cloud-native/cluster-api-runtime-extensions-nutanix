ARG MINDTHEGAP_VERSION=v1.24.0
ARG UUTILS_COREUTILS_VERSION=0.8.0

FROM --platform=${BUILDPLATFORM} ghcr.io/mesosphere/mindthegap:${MINDTHEGAP_VERSION} AS bundle_builder
# This gets called by goreleaser so the copy source has to be the path relative to the repo root.
RUN --mount=source=./hack/addons/helm-chart-bundler/repos.yaml,target=/repos.yaml \
    ["/ko-app/mindthegap", "create", "bundle", "--helm-charts-file=/repos.yaml", "--output-file=/tmp/helm-charts.tar"]

FROM --platform=${BUILDPLATFORM} alpine:3.21 AS uutils-downloader
ARG TARGETARCH
ARG UUTILS_COREUTILS_VERSION
RUN ARCH=$(case "${TARGETARCH}" in amd64) echo x86_64;; arm64) echo aarch64;; *) echo "${TARGETARCH}";; esac) && \
    wget -q -O - \
      "https://github.com/uutils/coreutils/releases/download/${UUTILS_COREUTILS_VERSION}/coreutils-${UUTILS_COREUTILS_VERSION}-${ARCH}-unknown-linux-musl.tar.gz" | \
    tar xz -C /tmp --strip-components=1

FROM scratch
# Add helm charts for the current version
ARG VERSION
COPY --from=bundle_builder /tmp/helm-charts.tar /charts/helm-charts-${VERSION}.tar
# TODO remove me as soon as its not needed to hold multiple versions of helm charts
COPY --from=ghcr.io/nutanix-cloud-native/caren-helm-reg:v0.14.6 /tmp/helm-charts.tar /charts/helm-charts-v0.14.6.tar
COPY --from=ghcr.io/nutanix-cloud-native/caren-helm-reg:v0.14.9 /tmp/helm-charts.tar /charts/helm-charts-v0.14.9.tar

# Add statically compiled cp to copy bundles to the mounted PVC at runtime.
# The uutils multicall binary dispatches based on argv[0], so naming it "cp" is sufficient.
COPY --from=uutils-downloader /tmp/coreutils /bin/cp
