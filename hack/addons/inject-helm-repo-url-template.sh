#!/usr/bin/env bash
# Copyright 2024 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# Injects the caren.helmAddonRepoURL include into each addon's RepositoryURL in
# helm-config.yaml so that override (e.g. OCI Harbor) > internal OCI repo > default HTTPS.
# Must run after generate-mindthegap-repofile so the repofile sees literal URLs.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

HELM_CONFIG="${GIT_REPO_ROOT}/charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml"

if [[ ! -f "${HELM_CONFIG}" ]]; then
  echo "error: ${HELM_CONFIG} not found" >&2
  exit 1
fi

yq -i '
  .data |= with_entries(
    .key as $k
    | .value |= (
        (. | from_yaml)
        | .RepositoryURL as $url
        | .RepositoryURL = ("{{ include \"caren.helmAddonRepoURL\" (dict \"addonKey\" \"" + $k + "\" \"defaultURL\" \"" + $url + "\" \"context\" .) }}")
        | to_yaml
      )
  )
' "${HELM_CONFIG}"
