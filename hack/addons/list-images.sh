#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

# shellcheck source=hack/common.sh
source "${SCRIPT_DIR}/../common.sh"

mapfile -t images < <(
  helm list-images ./charts/cluster-api-runtime-extensions-nutanix \
    --set-string image.tag="${CAREN_VERSION:-}" \
    --set-string helmRepositoryImage.tag="${CAREN_VERSION:-}"
)

function export_json_to_env() {
  INPUT_FILE="${1}"
  while IFS=$'\t\n' read -r LINE; do
    LINE="${LINE##[[:space:]][[:space:]]}"
    export "${LINE?}"
  done < <(
    gojq \
      --yaml-input \
      --yaml-output \
      --raw-output \
      --monochrome-output \
      '"CHART_NAME="+.helmCharts[0].name+"\n"+
           "CHART_VERSION="+.helmCharts[0].version+"\n"+
           "CHART_REPO="+.helmCharts[0].repo+"\n"+
           "CHART_VALUES_FILE="+(.helmCharts[0].valuesFile // "helm-values.yaml")+"\n"+
           "CHART_VALUES_INLINE="+((.helmCharts[0].valuesInline // "") | tostring)' \
      "${INPUT_FILE}" |
      tail -n +2
  )
}

function append_addon_images() {
  local kustomization_template="${1}"

  ASSETS_DIR="$(mktemp -d -p "${TMPDIR:-/tmp}")"
  trap_add "rm -rf ${ASSETS_DIR}" EXIT

  KUSTOMIZE_BASE_DIR="$(dirname "${kustomization_template}")"
  envsubst -no-unset -i "${kustomization_template}" -o "${ASSETS_DIR}/kustomization.yaml"
  if ls "${KUSTOMIZE_BASE_DIR}"/*.yaml &>/dev/null; then
    cp "${KUSTOMIZE_BASE_DIR}"/*.yaml "${ASSETS_DIR}"
  fi

  export_json_to_env "${ASSETS_DIR}/kustomization.yaml"

  echo >>"${ASSETS_DIR}/${CHART_VALUES_FILE}"
  gojq --yaml-output . <<<"${CHART_VALUES_INLINE:-}" >>"${ASSETS_DIR}/${CHART_VALUES_FILE}"

  mapfile -t addon_images < <(helm list-images "${CHART_NAME}" --chart-version "${CHART_VERSION}" --repo "${CHART_REPO}" --values "${ASSETS_DIR}/${CHART_VALUES_FILE}")
  images+=("${addon_images[@]}")
}

mapfile -t addon_kustomization_templates < <(find "${SCRIPT_DIR}/kustomize" -name "kustomization.yaml.tmpl")

for kustomization_template in "${addon_kustomization_templates[@]}"; do
  if [[ ${kustomization_template} == *"/aws-ccm/"* ]]; then
    for aws_ccm_version in 127 128 129 130; do
      aws_ccm_version_env_var="AWS_CCM_VERSION_${aws_ccm_version}"
      export AWS_CCM_VERSION="${!aws_ccm_version_env_var}"
      append_addon_images "${kustomization_template}"
    done
  else
    append_addon_images "${kustomization_template}"
  fi
done

echo "${images[*]}" | sort -u
