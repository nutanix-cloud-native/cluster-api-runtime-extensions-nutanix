ARG VERSION
ARG MINDTHEGAP_VERSION
FROM --platform=${BUILDPLATFORM} ghcr.io/mesosphere/mindthegap:${MINDTHEGAP_VERSION} as bundle_builder
# this gets called by goreleaser so the copy source has to be the path relative to the repo root.
RUN --mount=source=./hack/addons/mindthegap-helm-registry/repos.yaml,target=/repos.yaml \
    ["/ko-app/mindthegap", "create", "bundle", "--helm-charts-file=/repos.yaml", "--output-file=/tmp/helm-charts-${VERSION}.tar"]

FROM --platform=${TARGETPLATFORM} alpine
COPY --from=bundle_builder /tmp/helm-charts-${VERSION}.tar /charts/helm-charts-${VERSION}.tar
