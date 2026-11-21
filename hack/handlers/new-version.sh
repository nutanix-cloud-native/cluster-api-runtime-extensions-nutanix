#!/usr/bin/env bash

# Copyright 2026 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# Cuts handler version v(N+1) from the latest version vN: copies its code, bumps the handler names, snapshots the current
# API CRDs, registers the new version, and points the default ClusterClass templates at it. Refuses while vN is
# unreleased, because changes belong in the open version instead.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/../.."

readonly module=github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix

# open-version.sh fails both when every version is released and when there is no release tag; rule out the latter.
git describe --tags --match 'v*' --abbrev=0 >/dev/null
if open="$("${SCRIPT_DIR}/open-version.sh" 2>/dev/null)"; then
  echo "${open} is not released yet; change ${open} instead of cutting a new handler version" >&2
  exit 1
fi

n="$(find pkg/handlers -mindepth 1 -maxdepth 1 -type d -name 'v[0-9]*' | sed 's#.*/v##' | sort -n | tail -1)"
readonly old="v${n}" new="v$((n + 1))"
readonly src="pkg/handlers/${old}" dst="pkg/handlers/${new}"
if [ -e "${dst}" ]; then
  echo "${dst} already exists" >&2
  exit 1
fi

cp -R "${src}" "${dst}"
find "${dst}" -type f -exec perl -pi -e \
  "s#(${module}/pkg/handlers/)${old}([/\"])#\${1}${new}\${2}#g; s/${old}configpatch/${new}configpatch/g; s/ConfigVars${old}\"/ConfigVars${new}\"/g" \
  {} +
perl -pi -e "s/\b${old}\b/${new}/g" "${dst}/handlers.go"

rm -f "${dst}"/variables/crds/*.yaml
cp api/v1alpha1/crds/*.yaml "${dst}/variables/crds/"

perl -pi -e \
  "s#^(\t\"${module}/pkg/handlers/${old}\"\n)#\${1}\t\"${module}/pkg/handlers/${new}\"\n#; s#^(\t\t// new-version:entries)#\t\t${new}.AllHandlers(mgr),\n\${1}#" \
  pkg/handlers/versions.go

rule="$(awk -v start="        handlers-${old}:" '
  $0 == start { found = 1; print; next }
  found && /^        (handlers-v|# new-version:depguard)/ { exit }
  found { print }
' .golangci.yml | perl -pe "s/\b${old}\b/${new}/g")"
RULE="${rule}" perl -pi -e 'print "$ENV{RULE}\n" if /^        # new-version:depguard$/' .golangci.yml

perl -pi -e "s/${old}configpatch-gp/${new}configpatch-gp/g; s/configvars${old}-dv/configvars${new}-dv/g" \
  hack/examples/overlays/clusterclasses/*/kustomization.yaml.tmpl

echo "Created ${new} from ${old}"
