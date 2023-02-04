#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

if [ -z "${CALICO_VERSION-}" ]; then
  echo "Missing environment variable: CALICO_VERSION"
  exit 1
fi

CALICO_CNI_ASSETS_DIR="$(mktemp -d -p "${TMPDIR:-/tmp}")"
readonly CALICO_CNI_ASSETS_DIR
trap 'rm -rf ${CALICO_CNI_ASSETS_DIR}' EXIT

# The operator manifest in YAML format is 1226666 bytes. It turns out that much of that is whitespace. Converting the
# manifest to JSON without indentation allows us to remove most of the whitespace, reducing the size by more than half,
# to 527614 bytes.
#
# Some important notes:
# 1. The YAML manifest includes many documents, and the documents must become elements in a JSON array in order for the ClusterResourceController to [parse them](https://github.com/mesosphere/cluster-api//blob/65586de0080a960d085031de87ec627b2d606a6b/exp/addons/internal/controllers/clusterresourceset_helpers.go#L59). We create a JSON array with the --slurp flag.
# 2. The YAML manifest has some whitespace between YAML document markers (`---`), and these become `null` entries in the JSON array. This causes the ["SortForCreate" subroutine](https://github.com/mesosphere/cluster-api//blob/65586de0080a960d085031de87ec627b2d606a6b/exp/addons/internal/controllers/clusterresourceset_helpers.go#L84) of the ClusterResourceSet controller to misbehave. We remove these null entries using a filter expression.
# 3. If we indent the JSON document, it is nearly as large as the YAML document, at 1099093 bytes. We remove indentation with the --indent=0 flag.
curl -fsSL "https://docs.projectcalico.org/archive/${CALICO_VERSION}/manifests/tigera-operator.yaml" |
  gojq --yaml-input --slurp --indent=0 \
    '[ .[] | select( . != null ) | (select( .kind=="Namespace").metadata.labels += {"pod-security.kubernetes.io/enforce": "privileged", "pod-security.kubernetes.io/enforce-version": "latest"}) ]' \
    >"${CALICO_CNI_ASSETS_DIR}/tigera-operator.json"

kubectl create configmap tigera-operator --dry-run=client --output yaml \
  --from-file "${CALICO_CNI_ASSETS_DIR}/tigera-operator.json" \
  >"${GIT_REPO_ROOT}/pkg/addons/templates/cni/tigera-operator-configmap.yaml"
