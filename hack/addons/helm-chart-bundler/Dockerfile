ARG MINDTHEGAP_VERSION=v1.22.1

FROM --platform=${BUILDPLATFORM} ghcr.io/mesosphere/mindthegap:${MINDTHEGAP_VERSION} as bundle_builder
# This gets called by goreleaser so the copy source has to be the path relative to the repo root.
RUN --mount=source=./hack/addons/helm-chart-bundler/repos.yaml,target=/repos.yaml \
    ["/ko-app/mindthegap", "create", "bundle", "--helm-charts-file=/repos.yaml", "--output-file=/tmp/helm-charts.tar"]

FROM --platform=${TARGETPLATFORM} busybox:1.37.0-musl as static-busybox

FROM --platform=${TARGETPLATFORM} scratch
# Add helm charts for the current version
ARG VERSION
COPY --from=bundle_builder /tmp/helm-charts.tar /charts/helm-charts-${VERSION}.tar
# TODO remove me as soon as its not needed to hold multiple versions of helm charts
COPY --from=ghcr.io/nutanix-cloud-native/caren-helm-reg:v0.14.6 /tmp/helm-charts.tar /charts/helm-charts-v0.14.6.tar
COPY --from=ghcr.io/nutanix-cloud-native/caren-helm-reg:v0.14.9 /tmp/helm-charts.tar /charts/helm-charts-v0.14.9.tar

# Add statically compiled cp to the image used to copy bundles to the mounted PVC at runtime.
COPY --from=static-busybox /bin/cp /bin/cp
