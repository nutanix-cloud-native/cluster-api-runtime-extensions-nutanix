ARG MINDTHEGAP_VERSION=v1.14.4
FROM --platform=${BUILDPLATFORM} ghcr.io/mesosphere/mindthegap:${MINDTHEGAP_VERSION} as bundle_builder
# this gets called by goreleaser so the copy source has to be the path relative to the repo root.
RUN --mount=source=./hack/addons/mindthegap-helm-registry/repos.yaml,target=/repos.yaml \
    ["/ko-app/mindthegap", "create", "bundle", "--helm-charts-file=/repos.yaml", "--output-file=/tmp/helm-charts.tar"]

FROM --platform=${TARGETPLATFORM} alpine:3.20.3
ARG VERSION
COPY --from=bundle_builder /tmp/helm-charts.tar /charts/helm-charts-${VERSION}.tar
COPY --from=bundle_builder /ko-app/mindthegap /usr/bin/mindthegap
VOLUME /certs
ENTRYPOINT /usr/bin/mindthegap
CMD [ "serve", "bundle", \
      "--bundle=/tmp/helm-charts.tar", \
      "--listen-port=5000", \
      "--listen-address=0.0.0.0", \
      "--tls-private-key-file=/certs/tls.key", \
      "--tls-cert-file=/certs/tls.crt" ]
