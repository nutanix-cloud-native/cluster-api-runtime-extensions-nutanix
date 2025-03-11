#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_NAME="$(basename "${0}")"
readonly SCRIPT_NAME

declare -r KUBEADM_INIT_FILE="/run/kubeadm/kubeadm.yaml"
declare -r KUBE_VIP_MANIFEST_FILE="/etc/kubernetes/manifests/kube-vip.yaml"

function use_super_admin_conf {
  if [[ -f ${KUBEADM_INIT_FILE} && -f ${KUBE_VIP_MANIFEST_FILE} ]]; then
    sed -i 's#path: /etc/kubernetes/admin.conf#path: /etc/kubernetes/super-admin.conf#' \
      /etc/kubernetes/manifests/kube-vip.yaml
  fi
}

function use_admin_conf() {
  if [[ -f ${KUBEADM_INIT_FILE} && -f ${KUBE_VIP_MANIFEST_FILE} ]]; then
    sed -i 's#path: /etc/kubernetes/super-admin.conf#path: /etc/kubernetes/admin.conf#' \
      /etc/kubernetes/manifests/kube-vip.yaml
  fi
}

function set_host_aliases() {
  echo "127.0.0.1   kubernetes" >>/etc/hosts
}

function print_usage {
  cat >&2 <<EOF
Usage: ${SCRIPT_NAME} [set-host-aliases|use-super-admin.conf|use-admin.conf]
EOF
}

function run_cmd() {
  while [ $# -gt 0 ]; do
    case $1 in
    use-super-admin.conf)
      use_super_admin_conf
      ;;
    use-admin.conf)
      use_admin_conf
      ;;
    set-host-aliases)
      set_host_aliases
      ;;
    -h | --help)
      print_usage
      exit
      ;;
    *)
      echo "invalid argument"
      exit 1
      ;;
    esac
    shift
  done
}

run_cmd "$@"
