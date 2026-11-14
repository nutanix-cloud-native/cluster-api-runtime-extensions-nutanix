#!/usr/bin/env bash

# Copyright 2026 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# Fails if the working tree of the current git repository changes what the handlers of the last release produce:
#   - a golden or corpus file of that release is modified or deleted;
#   - a file of a released pkg/handlers/vN directory is modified or deleted, unless FROZEN_HANDLER_OVERRIDE=true;
#   - a handler name registered in that release is no longer registered.
# Names listed in pkg/handlers/removed-handlers.txt may be unregistered, and their golden files and version directories
# deleted.

set -euo pipefail
IFS=$'\n\t'

cd "$(git rev-parse --show-toplevel)"

readonly record=pkg/handlers/removed-handlers.txt
readonly testdata=pkg/handlers/golden/testdata
release="${LAST_RELEASE:-$(git describe --tags --match 'v*' --abbrev=0)}"
readonly release

removed=""
if [ -f "${record}" ]; then
  removed="$(awk '!/^#/ && NF {print $1}' "${record}")"
fi
readonly removed

is_removed() {
  grep -qFx -- "${1}" <<<"${removed}"
}

# names <path> [<release>] prints the handler names in Go files under path, in the working tree or at the release.
# ponytail: names are found by matching string literals; a generated list of registered names would be exact.
names() {
  local -r pattern='"[A-Za-z]+(v[0-9]+configpatch|ConfigVars(v[0-9]+)?)"'
  if [ $# -gt 1 ]; then
    git grep -ohE "${pattern}" "${2}" -- "${1}/*.go" ':!*_test.go' || true
  else
    git grep --untracked -ohE "${pattern}" -- "${1}/*.go" ':!*_test.go' || true
  fi | tr -d '"' | sort -u
}

failed=0
fail() {
  echo "${1}" >&2
  failed=1
}

changed="$(git diff --name-only --no-renames --diff-filter=MDT "${release}" -- "${testdata}")"
for f in ${changed}; do
  name="${f#"${testdata}/golden/"}"
  if [ "${name}" != "${f}" ] && is_removed "${name%%/*}"; then
    continue
  fi
  fail "${f} is released golden test data and must not change"
done

dirs="$(git ls-tree -d --name-only "${release}" pkg/handlers/ | grep -E '^pkg/handlers/v[0-9]+$' || true)"
for dir in ${dirs}; do
  dir_names="$(names "${dir}" "${release}")"
  retired="$([ -n "${dir_names}" ] && echo true || echo false)"
  for n in ${dir_names}; do
    is_removed "${n}" || retired=false
  done
  if "${retired}" || [ "${FROZEN_HANDLER_OVERRIDE:-false}" = true ]; then
    continue
  fi
  changed="$(git diff --name-only --no-renames --diff-filter=MDT "${release}" -- "${dir}")"
  for f in ${changed}; do
    fail "${f} is released handler code and must not change without the frozen-handler-override label"
  done
done

registered="$(names pkg/handlers)"
for n in $(names pkg/handlers "${release}"); do
  if ! is_removed "${n}" && ! grep -qFx -- "${n}" <<<"${registered}"; then
    fail "${n} was registered in ${release} and is no longer registered; list it in ${record} to remove it"
  fi
done

if [ "${failed}" -ne 0 ]; then
  exit 1
fi
echo "No released handler output or registration changed since ${release}"
