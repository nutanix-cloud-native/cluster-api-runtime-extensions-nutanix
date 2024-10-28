ARG MINDTHEGAP_VERSION=v1.14.4
FROM --platform=${BUILDPLATFORM} ghcr.io/mesosphere/mindthegap:${MINDTHEGAP_VERSION} as bundle_builder
# this gets called by goreleaser so the copy source has to be the path relative to the repo root.
RUN --mount=source=./hack/addons/mindthegap-helm-registry/repos.yaml,target=/repos.yaml \
    ["/ko-app/mindthegap", "create", "bundle", "--helm-charts-file=/repos.yaml", "--output-file=/tmp/helm-charts.tar"]

FROM --platform=${TARGETPLATFORM} ghcr.io/mesosphere/mindthegap:${MINDTHEGAP_VERSION} as mindthegap

FROM --platform=${TARGETPLATFORM} alpine:3.20.3
# Add mindthegap binary that matches TARGETPLATFORM
COPY --from=mindthegap /ko-app/mindthegap /usr/bin/mindthegap
# Add helm charts for the current version
ARG VERSION
COPY --from=bundle_builder /tmp/helm-charts.tar /charts/helm-charts-${VERSION}.tar
# TODO remove me as soon as its not needed to hold multiple versions of helm charts
COPY --from=ghcr.io/nutanix-cloud-native/caren-helm-reg:v0.14.6 /tmp/helm-charts.tar /charts/helm-charts-v0.14.6.tar
COPY --from=ghcr.io/nutanix-cloud-native/caren-helm-reg:v0.14.9 /tmp/helm-charts.tar /charts/helm-charts-v0.14.9.tar
VOLUME /certs
ENTRYPOINT /usr/bin/mindthegap
