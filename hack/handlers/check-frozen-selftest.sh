#!/usr/bin/env bash

# Copyright 2026 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# Runs check-frozen.sh against changes to a throwaway repository with a v1.0.0 release.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly CHECK="${SCRIPT_DIR}/check-frozen.sh"

# The throwaway repository must not pick up user or system git config such as commit signing.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=selftest GIT_AUTHOR_EMAIL=selftest@example.com
export GIT_COMMITTER_NAME=selftest GIT_COMMITTER_EMAIL=selftest@example.com

tmp="$(mktemp -d)"
readonly tmp
trap 'rm -rf "${tmp}"' EXIT

readonly base="${tmp}/base"
readonly golden=pkg/handlers/golden/testdata/golden/fooClusterv1configpatch
readonly corpus=pkg/handlers/golden/testdata/corpus/foo
mkdir -p "${base}/pkg/handlers/v1" "${base}/pkg/handlers/foo" "${base}/${golden}/foo" "${base}/${corpus}"
echo 'const name = "fooClusterv1configpatch"' >"${base}/pkg/handlers/v1/handler.go"
echo 'const name = "FooClusterConfigVars"' >"${base}/pkg/handlers/foo/variables.go"
echo golden >"${base}/${golden}/foo/input.yaml"
echo corpus >"${base}/${corpus}/input.yaml"
git -C "${base}" init -q
git -C "${base}" add -A
git -C "${base}" commit -q -m release
git -C "${base}" tag v1.0.0

failures=0

# expect <exit status> <case> <override> <shell commands that change the clone>
expect() {
  local -r want="${1}" name="${2}" override="${3}" change="${4}"
  local -r dir="${tmp}/case-${RANDOM}${RANDOM}"
  git clone -q "${base}" "${dir}"
  (cd "${dir}" && eval "${change}")
  local got=0
  (cd "${dir}" && FROZEN_HANDLER_OVERRIDE="${override}" "${CHECK}" >/dev/null 2>&1) || got=$?
  if [ "${got}" != "${want}" ]; then
    echo "FAIL: ${name}: exit status ${got}, want ${want}"
    failures=$((failures + 1))
  fi
}

expect 0 "no change" false ":"
expect 1 "modified golden file" false "echo changed >>${golden}/foo/input.yaml"
expect 1 "modified golden file with override" true "echo changed >>${golden}/foo/input.yaml"
expect 1 "deleted corpus file" false "rm ${corpus}/input.yaml"
expect 1 "renamed golden file" false "git mv ${golden}/foo/input.yaml ${golden}/foo/renamed.yaml"
expect 1 "renamed released version file" false "git mv pkg/handlers/v1/handler.go pkg/handlers/v1/renamed.go"
expect 1 "modified released version file" false "echo '// changed' >>pkg/handlers/v1/handler.go"
expect 0 "modified released version file with override" true "echo '// changed' >>pkg/handlers/v1/handler.go"
expect 1 "removed handler name" true "rm -r pkg/handlers/v1"
expect 1 "removed discovery name" false "rm pkg/handlers/foo/variables.go"
expect 0 "removed handler name in the record with its directories deleted" false \
  "rm -r pkg/handlers/v1 ${golden} && echo 'fooClusterv1configpatch v1.0.0' >pkg/handlers/removed-handlers.txt"
expect 1 "removed handler name in the record with its corpus deleted" false \
  "rm -r pkg/handlers/v1 ${golden} ${corpus} && echo 'fooClusterv1configpatch v1.0.0' >pkg/handlers/removed-handlers.txt"
expect 0 "added golden file" false "echo new >${golden}/foo/new.yaml"

if [ "${failures}" -ne 0 ]; then
  exit 1
fi
echo "check-frozen.sh self-test passed"
