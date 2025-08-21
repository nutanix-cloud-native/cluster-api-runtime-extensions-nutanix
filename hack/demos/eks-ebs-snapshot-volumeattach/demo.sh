#!/usr/bin/env bash

# Copyright 2025 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
IFS=$'\n\t'

# This script demonstrates:
# 1. Creating an EKS cluster via ClusterClass
# 2. Creating a PersistentVolumeClaim (PVC)
# 3. Populating it with data via a Pod
# 4. Taking a VolumeSnapshot
# 5. Restoring the snapshot to a new PVC
# 6. Attaching the restored volume to a node via VolumeAttachment
# 7. Validating the data is present on the attached volume

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
cd "${SCRIPT_DIR}"

if [[ -z ${EKS_CLUSTER_NAME:-} ]]; then
  EKS_CLUSTER_NAME="$(kubectl get clusters -l caren.nutanix.com/demo-name="EBSSnapshotVolumeAttach" -o custom-columns=NAME:.metadata.name --no-headers)"
  if [[ -z ${EKS_CLUSTER_NAME} ]]; then
    EKS_CLUSTER_NAME="eks-volumeattach-$(head /dev/urandom | tr -dc a-z0-9 | head -c6)"
  fi
fi
export EKS_CLUSTER_NAME
echo "Using EKS cluster name: ${EKS_CLUSTER_NAME}"

echo
echo "Step 1: Create an EKS cluster via ClusterClass"
envsubst --no-unset -i eks-test.yaml | kubectl apply --server-side -f -
kubectl wait --for=condition=Ready cluster/"${EKS_CLUSTER_NAME}" --timeout=20m

echo "Cluster is ready, getting kubeconfig"
kubectl wait --for=create secret "${EKS_CLUSTER_NAME}-user-kubeconfig" --timeout=30s
EKS_KUBECONFIG="$(mktemp -p "${TMPDIR:-/tmp}")"
kubectl get secrets "${EKS_CLUSTER_NAME}-user-kubeconfig" -oyaml |
  gojq --yaml-input -r '.data.value | @base64d' >"${EKS_KUBECONFIG}"
export KUBECONFIG="${EKS_KUBECONFIG}"
echo "Using kubeconfig: ${EKS_KUBECONFIG}"

STORAGE_CLASS="${STORAGE_CLASS:-aws-ebs-default}"
STORAGE_CLASS_IMMEDIATE="${STORAGE_CLASS_IMMEDIATE:-aws-ebs-immediate-binding}"
VOLUME_SNAPSHOT_CLASS="${VOLUME_SNAPSHOT_CLASS:-ebs-snapclass}"
ORIGINAL_PVC="${ORIGINAL_PVC:-pvc-demo-original}"
RESTORED_PVC="${RESTORED_PVC:-pvc-demo-restored}"
SNAPSHOT_NAME="${SNAPSHOT_NAME:-pvc-demo-snapshot}"
DATA_POD="${DATA_POD:-data-writer}"
RESTORE_POD="${RESTORE_POD:-data-reader}"

NAMESPACE="$(kubectl get namespace -l caren.nutanix.com/demo-name="EBSSnapshotVolumeAttach" -o custom-columns=NAME:.metadata.name --no-headers)"
if [[ -z ${NAMESPACE} ]]; then
  NAMESPACE="ebs-demo-$(head /dev/urandom | tr -dc a-z0-9 | head -c6)"
  cat <<EOF | kubectl apply --server-side -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
  labels:
    caren.nutanix.com/demo-name: "EBSSnapshotVolumeAttach"
EOF
fi
echo "Using namespace: ${NAMESPACE}"

echo
echo "Step 2: Create a PersistentVolumeClaim"
cat <<EOF | kubectl apply --server-side -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ${ORIGINAL_PVC}
  namespace: ${NAMESPACE}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ${STORAGE_CLASS}
EOF
kubectl -n "${NAMESPACE}" get pvc "${ORIGINAL_PVC}"

echo
echo "Step 3: Create a Pod to write data to the PVC"
cat <<EOF | kubectl apply --server-side -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${DATA_POD}
  namespace: ${NAMESPACE}
spec:
  restartPolicy: Never
  containers:
  - name: writer
    image: busybox
    command: ["/bin/sh", "-c"]
    args:
      - echo "Hello from original PVC!" > /data/hello.txt; sleep 10
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: ${ORIGINAL_PVC}
EOF
kubectl -n "${NAMESPACE}" get pod "${DATA_POD}"

echo "Waiting for PVC to be bound..."
kubectl -n "${NAMESPACE}" wait --for=jsonpath='{.status.phase}'=Bound pvc/"${ORIGINAL_PVC}" --timeout=120s
kubectl -n "${NAMESPACE}" get pvc "${ORIGINAL_PVC}"

echo "Waiting for data writer pod to complete..."
kubectl -n "${NAMESPACE}" wait --for=jsonpath='{.status.phase}'=Succeeded pod/"${DATA_POD}" --timeout=120s
kubectl -n "${NAMESPACE}" get pod "${DATA_POD}"

echo
echo "Step 4: Create a VolumeSnapshotClass"
cat <<EOF | kubectl apply --server-side -f -
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: ${VOLUME_SNAPSHOT_CLASS}
driver: ebs.csi.aws.com
deletionPolicy: Delete
EOF
kubectl -n "${NAMESPACE}" get volumesnapshotclass "${VOLUME_SNAPSHOT_CLASS}"

echo
echo "Step 5: Take a snapshot of the PVC"
cat <<EOF | kubectl apply --server-side -f -
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: ${SNAPSHOT_NAME}
  namespace: ${NAMESPACE}
spec:
  volumeSnapshotClassName: ${VOLUME_SNAPSHOT_CLASS}
  source:
    persistentVolumeClaimName: ${ORIGINAL_PVC}
EOF
kubectl -n "${NAMESPACE}" get volumesnapshot "${SNAPSHOT_NAME}"

echo "Waiting for VolumeSnapshot to be ready..."
kubectl -n "${NAMESPACE}" wait --for=jsonpath='{.status.readyToUse}'=true volumesnapshot/"${SNAPSHOT_NAME}" --timeout=120s
kubectl -n "${NAMESPACE}" get volumesnapshot "${SNAPSHOT_NAME}"

echo
echo "Step 6: Create standard performance VolumeAttributesClass"
cat <<EOF | kubectl apply --server-side -f -
apiVersion: storage.k8s.io/v1beta1
kind: VolumeAttributesClass
metadata:
  name: ${STORAGE_CLASS_IMMEDIATE}-standard-performance
driverName: ebs.csi.aws.com
parameters:
  type: gp3
  iops: "3000"
  throughput: "125"
EOF
kubectl -n "${NAMESPACE}" get volumeattributesclass "${STORAGE_CLASS_IMMEDIATE}-standard-performance" -oyaml

echo
echo "Step 7: Create enhanced performance VolumeAttributesClass"
cat <<EOF | kubectl apply --server-side -f -
apiVersion: storage.k8s.io/v1beta1
kind: VolumeAttributesClass
metadata:
  name: ${STORAGE_CLASS_IMMEDIATE}-enhanced-performance
driverName: ebs.csi.aws.com
parameters:
  type: gp3
  iops: "4000"
  throughput: "130"
EOF
kubectl -n "${NAMESPACE}" get volumeattributesclass "${STORAGE_CLASS_IMMEDIATE}-enhanced-performance" -oyaml

echo
echo "Step 8: Restore the snapshot to a new PVC with standard performance"
cat <<EOF | kubectl apply --server-side -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ${RESTORED_PVC}
  namespace: ${NAMESPACE}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: ${STORAGE_CLASS_IMMEDIATE}
  volumeAttributesClassName: ${STORAGE_CLASS_IMMEDIATE}-standard-performance
  dataSource:
    name: ${SNAPSHOT_NAME}
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
EOF
kubectl -n "${NAMESPACE}" get pvc "${RESTORED_PVC}"

echo "Waiting for restored PVC to be bound..."
kubectl -n "${NAMESPACE}" wait --for=jsonpath='{.status.phase}'=Bound pvc/"${RESTORED_PVC}" --timeout=120s
kubectl -n "${NAMESPACE}" get pvc "${RESTORED_PVC}"

echo "Waiting for PVC to have standard performance volume attributes..."
kubectl -n "${NAMESPACE}" wait --for=jsonpath='{.status.currentVolumeAttributesClassName}'="${STORAGE_CLASS_IMMEDIATE}-standard-performance" pvc/"${RESTORED_PVC}" --timeout=120s

echo
echo "Step 9: Attach the restored volume to a node via VolumeAttachment"
# Find the PV backing the restored PVC
RESTORED_PV="$(kubectl -n "${NAMESPACE}" get pvc "${RESTORED_PVC}" -o jsonpath='{.spec.volumeName}')"
# Find a node to attach to
NODE_NAME=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')

ATTACHMENT_NAME="attach-${RESTORED_PV}"

cat <<EOF | kubectl apply --server-side -f -
apiVersion: storage.k8s.io/v1
kind: VolumeAttachment
metadata:
  name: ${ATTACHMENT_NAME}
spec:
  attacher: ebs.csi.aws.com
  nodeName: ${NODE_NAME}
  source:
    persistentVolumeName: ${RESTORED_PV}
EOF
kubectl -n "${NAMESPACE}" get volumeattachment "${ATTACHMENT_NAME}"

echo "Waiting for VolumeAttachment to be attached..."
kubectl wait --for=jsonpath='{.status.attached}'=true volumeattachment/"${ATTACHMENT_NAME}" --timeout=120s
kubectl -n "${NAMESPACE}" get volumeattachment "${ATTACHMENT_NAME}"

echo
echo "Step 10: Show that the restored volume is attached directly to the node with the correct data"
ATTACHMENT_DEVICE_PATH=$(kubectl -n "${NAMESPACE}" get volumeattachment "${ATTACHMENT_NAME}" -o jsonpath='{.status.attachmentMetadata.devicePath}')
kubectl debug "node/${NODE_NAME}" --image=ubuntu -it --profile=sysadmin --quiet -- \
  bash -ec "chroot /host bash -ec \"mkdir -p /tmp/attached; mount \"${ATTACHMENT_DEVICE_PATH}\" /tmp/attached; cat /tmp/attached/hello.txt; umount /tmp/attached\""

echo
echo "Step 11: Patch the restored PVC to use enhanced performance via VolumeAttributesClass"
kubectl -n "${NAMESPACE}" patch pvc "${RESTORED_PVC}" --type='merge' -p='{"spec":{"volumeAttributesClassName":"'"${STORAGE_CLASS_IMMEDIATE}"'-enhanced-performance"}}'

echo "Waiting for PVC to have enhanced performance volume attributes..."
kubectl -n "${NAMESPACE}" wait --for=jsonpath='{.status.currentVolumeAttributesClassName}'="${STORAGE_CLASS_IMMEDIATE}-enhanced-performance" pvc/"${RESTORED_PVC}" --timeout=120s

echo
echo
echo "When you are ready, cleanup the resources created by this demo by running:"
echo
echo "kubectl --kubeconfig=${KUBECONFIG} delete namespace ${NAMESPACE}"
echo "kubectl delete cluster ${EKS_CLUSTER_NAME}"
