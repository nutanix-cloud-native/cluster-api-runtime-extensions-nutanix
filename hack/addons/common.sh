#!/usr/bin/env bash

# images_file_from_configmap_yaml_manifest extracts the images from YAML manifests embedded in configmaps,
# as used in ClusterResourceSets, and saves it to a file.
images_file_from_configmap_yaml_manifest() {
  local -r configmap_file="${1}"
  local -r name="${2}"
  local -r version="${3}"
  local -r images_file="${4}"

  # shellcheck disable=SC2207 # mapfile doesn't exist on macos
  IFS=' ' all_images=($(
    gojq --yaml-input --raw-output '.data[]?+"---"' "${configmap_file}" |
      gojq --raw-output --yaml-input \
        '[.spec.template.spec.containers[]?.image,.spec.template.spec.initContainers[]?.image] | unique | .[]'
  ))

  # sort and remove duplicates
  local -r unique_images=$(for img in "${all_images[@]}"; do echo "${img}"; done | sort -u)

  write_images_file "${unique_images}" "${name}" "${version}" "${images_file}"
}

# images_file_from_configmap_yaml_manifest extracts the images from JSON manifests embedded in configmaps,
# as used in ClusterResourceSets, and saves it to a file.
images_file_from_configmap_json_manifest() {
  local -r configmap_file="${1}"
  local -r name="${2}"
  local -r version="${3}"
  local -r images_file="${4}"

  # shellcheck disable=SC2207 # mapfile doesn't exist on macos
  IFS=' ' all_images=($(
    gojq --yaml-input --raw-output '.data[]?' "${configmap_file}" |
      gojq --raw-output \
        ' .[] | [.spec.template.spec.containers[]?.image,.spec.template.spec.initContainers[]?.image] | unique | .[]'
  ))

  # sort and remove duplicates
  local -r unique_images=$(for img in "${all_images[@]}"; do echo "${img}"; done | sort -u)

  write_images_file "${unique_images}" "${name}" "${version}" "${images_file}"
}

# write_images_file writes the images to a yaml file.
write_images_file() {
  local -r images="${1}"
  local -r name="${2}"
  local -r version="${3}"
  local -r images_file="${4}"

  cat "${GIT_REPO_ROOT}/hack/license-header.yaml.txt" >"${images_file}"

  # shellcheck disable=SC2129 # prefer individual lines for readability
  echo "${name}:" >>"${images_file}"
  echo "  version: ${version}" >>"${images_file}"
  echo "  images:" >>"${images_file}"

  while IFS=' ' read -r image; do
    echo "  - ${image}" >>"${images_file}"
  done <<<"${images}"
}

# merge_images_file merges the images from a yaml file $1 into another file $2.
merge_images_file() {
  local -r file="${1}"
  local -r target_file="${2}"

  # shellcheck disable=SC2016 # single quotes are here for yq
  yq ea -i '. as $item ireduce ({}; . * $item )' "${target_file}" "${file}"
  yq -i -P 'sort_keys(.)' "${target_file}"

  # remove comments from the merged file and then add back the license header
  yq -i '... comments=""' "${target_file}"
  trap_add "rm -rf ${target_file}.tmp" EXIT
  cat "${GIT_REPO_ROOT}/hack/license-header.yaml.txt" "${target_file}" >"${target_file}.tmp"
  mv "${target_file}.tmp" "${target_file}"
}

# images_file_for_calico_cni extracts the images based on the calico operator version.
# Unlike other addons, only the operator image is present in the configmap.
images_file_for_calico_cni() {
  local -r configmap_file="${1}"
  local -r name="${2}"
  local -r version="${3}"
  local -r images_file="${4}"

  local -r calico_operator_version="$(grep --only-matching --perl-regexp "(?<=quay.io/tigera/operator):\Kv.*?(?=\"|\$)" "${configmap_file}")"
  local -r calico_versions="$(curl -s https://raw.githubusercontent.com/tigera/operator/"${calico_operator_version}"/config/calico_versions.yml)"

  local -a calico_images=("quay.io/tigera/operator:${calico_operator_version}")
  calico_images+=("calico/cni:$(echo "${calico_versions}" | gojq --yaml-input --raw-output '.components["calico/cni"].version')")
  calico_images+=("calico/kube-controllers:$(echo "${calico_versions}" | gojq --yaml-input --raw-output '.components["calico/kube-controllers"].version')")
  calico_images+=("calico/node:$(echo "${calico_versions}" | gojq --yaml-input --raw-output '.components["calico/node"].version')")
  calico_images+=("calico/apiserver:$(echo "${calico_versions}" | gojq --yaml-input --raw-output '.components["calico/apiserver"].version')")
  calico_images+=("calico/pod2daemon-flexvol:$(echo "${calico_versions}" | gojq --yaml-input --raw-output .components.flexvol.version)")
  calico_images+=("calico/typha:$(echo "${calico_versions}" | gojq --yaml-input --raw-output .components.typha.version)")
  calico_images+=("calico/csi:$(echo "${calico_versions}" | gojq --yaml-input --raw-output '.components["calico/csi"].version')")
  calico_images+=("calico/node-driver-registrar:$(echo "${calico_versions}" | gojq --yaml-input --raw-output '.components["csi-node-driver-registrar"].version')")
  # The `calico/ctl` image contains the Calico CLI, which can be useful for debugging.
  calico_images+=("calico/ctl:$(echo "${calico_versions}" | gojq --yaml-input --raw-output '.components["calico/cni"].version')")

  # sort and remove duplicates
  local -r unique_images=$(for img in "${calico_images[@]}"; do echo "${img}"; done | sort -u)

  write_images_file "${unique_images}" "${name}" "${version}" "${images_file}"
}
