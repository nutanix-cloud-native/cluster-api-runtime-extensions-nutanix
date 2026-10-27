#!/usr/bin/env bash

# Copyright 2026 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# Prints the open handler version, vN, where N is the highest handler version in pkg/handlers and is not in the last
# release. Exits non-zero if every handler version has been released, because released golden output must not change.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/../.."

last_release="$(git describe --tags --match 'v*' --abbrev=0)"
highest="$(grep -rhoE --include='*.go' '[A-Za-z]+v[0-9]+configpatch' pkg/handlers |
  sed -E 's/.*v([0-9]+)configpatch$/\1/' | sort -n | tail -1)"

if git grep -q "v${highest}configpatch" "${last_release}" -- pkg/handlers; then
  echo "v${highest} was released in ${last_release}; cut a new handler version first" >&2
  exit 1
fi

echo "v${highest}"
