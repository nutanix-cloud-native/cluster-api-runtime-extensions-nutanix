#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

if [ -z "${KUBE_VIP_VERSION:-}" ]; then
  echo "Missing argument: KUBE_VIP_VERSION"
  exit 1
fi

readonly ASSETS_DIR="hack/examples/files"
readonly FILE_NAME="kube-vip.yaml"

# shellcheck disable=SC2016 # Single quotes are required for the gojq expression.
docker container run --rm ghcr.io/kube-vip/kube-vip:"${KUBE_VIP_VERSION}" \
  manifest pod \
  --arp \
  --address='127.0.0.1' \
  --controlplane \
  --leaderElection \
  --leaseDuration=15 \
  --leaseRenewDuration=10 \
  --leaseRetry=2 \
  --prometheusHTTPServer='' |
  gojq --yaml-input --yaml-output \
    'del(.metadata.creationTimestamp, .status) |
     .spec.containers[].imagePullPolicy |= "IfNotPresent" |
     (.spec.containers[0].env[] | select(.name == "port").value) |= "{{ .Port }}" |
     (.spec.containers[0].env[] | select(.name == "address").value) |= "{{ .Address }}"
    ' >"${ASSETS_DIR}/${FILE_NAME}"

# add 8 spaces to each line so that the kustomize template can be properly indented
sed -i -e 's/^/        /' "${ASSETS_DIR}/${FILE_NAME}"
