#!/bin/sh
# Computes kubelet kubeReserved (CPU, memory) and a hard eviction threshold from
# the node's actual capacity, then writes a strategic-merge kubelet patch that
# kubeadm consumes. Inputs can be overridden via environment for testing.
#
# The tiered CPU and memory percentages mirror GKE's node-allocatable formula
# (https://cloud.google.com/kubernetes-engine/docs/concepts/cluster-architecture#node_allocatable):
# reserve a larger fraction on small nodes and a progressively smaller fraction
# as the node grows. EKS applies the same idea in its bootstrap scripts.
set -eu

patch_dir="${CAREN_KUBELET_PATCH_DIR:-/etc/kubernetes/patches}"
patch_file="${patch_dir}/kubeletconfiguration50+strategic.json"

cores="${CAREN_NODE_CPU_CORES:-$(grep -c '^processor' /proc/cpuinfo)}"
mem_kib="${CAREN_NODE_MEMORY_KIB:-$(awk '/^MemTotal:/ {print $2}' /proc/meminfo)}"

if ! [ "${cores}" -ge 1 ] 2>/dev/null; then
	echo "compute-reservations: could not determine CPU core count" >&2
	exit 1
fi
if ! [ "${mem_kib}" -ge 1 ] 2>/dev/null; then
	echo "compute-reservations: could not determine total memory" >&2
	exit 1
fi

# CPU reservation in millicores. Tiered: 6% of the 1st core, 1% of the 2nd,
# 0.5% of cores 3-4, 0.25% beyond 4. Computed in tenths of a millicore to keep
# integer arithmetic, then rounded to the nearest millicore.
tenths=0
if [ "${cores}" -ge 1 ]; then tenths=$((tenths + 600)); fi
if [ "${cores}" -ge 2 ]; then tenths=$((tenths + 100)); fi
n34=$((cores - 2))
if [ "${n34}" -lt 0 ]; then n34=0; fi
if [ "${n34}" -gt 2 ]; then n34=2; fi
tenths=$((tenths + n34 * 50))
n5=$((cores - 4))
if [ "${n5}" -lt 0 ]; then n5=0; fi
tenths=$((tenths + n5 * 25))
cpu_milli=$(((tenths + 5) / 10))

# Memory reservation in MiB. Tiers use 1Gi = 1024 MiB boundaries:
# 255Mi below 1Gi; else 25% of first 4Gi + 20% of next 4Gi + 10% of next 8Gi
# + 6% of next 112Gi + 2% above 128Gi. Per-tier floor (integer division).
total_mib=$((mem_kib / 1024))
if [ "${total_mib}" -lt 1024 ]; then
	mem_mib=255
else
	mem_mib=0
	t=${total_mib}
	if [ "${t}" -gt 4096 ]; then t=4096; fi
	mem_mib=$((mem_mib + t * 25 / 100))
	t=$((total_mib - 4096))
	if [ "${t}" -lt 0 ]; then t=0; fi
	if [ "${t}" -gt 4096 ]; then t=4096; fi
	mem_mib=$((mem_mib + t * 20 / 100))
	t=$((total_mib - 8192))
	if [ "${t}" -lt 0 ]; then t=0; fi
	if [ "${t}" -gt 8192 ]; then t=8192; fi
	mem_mib=$((mem_mib + t * 10 / 100))
	t=$((total_mib - 16384))
	if [ "${t}" -lt 0 ]; then t=0; fi
	if [ "${t}" -gt 114688 ]; then t=114688; fi
	mem_mib=$((mem_mib + t * 6 / 100))
	t=$((total_mib - 131072))
	if [ "${t}" -lt 0 ]; then t=0; fi
	mem_mib=$((mem_mib + t * 2 / 100))
fi

mkdir -p "${patch_dir}"
cat >"${patch_file}" <<EOF
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
kubeReserved:
  cpu: "${cpu_milli}m"
  memory: "${mem_mib}Mi"
evictionHard:
  memory.available: "100Mi"
EOF
