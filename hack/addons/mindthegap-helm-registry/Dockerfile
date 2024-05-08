FROM ghcr.io/mesosphere/mindthegap:v1.13.4
# this gets called by goreleaser so the copy source has to be the path relative to the repo root.
RUN --mount=source=./hack/addons/mindthegap-helm-registry/repos.yaml,target=/repos.yaml ["/ko-app/mindthegap", "create", "helm-bundle", "--helm-charts-file=/repos.yaml", "--output-file=/tmp/helm-charts.tar"]
VOLUME /certs/tls.crt
VOLUME /certs/tls.key
CMD ["serve", "bundle", "--bundle=/tmp/helm-charts.tar", "--listen-port=5000", "--listen-address=0.0.0.0", "--tls-private-key-file=/certs.tls.key", "--tls-cert-file=/certs/tls.crt"]
