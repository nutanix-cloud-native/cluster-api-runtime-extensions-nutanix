#!/usr/bin/env bash

GIT_REPO_ROOT="$(git rev-parse --show-toplevel)"
export GIT_REPO_ROOT

trap_add() {
  local -r sig="${2:?Signal required}"
  local -r hdls="$(trap -p "${sig}" | cut -f2 -d \')"
  # shellcheck disable=SC2064 # Quotes are required here to properly expand when adding the new trap.
  trap "${hdls}${hdls:+;}${1:?Handler required}" "${sig}"
}
