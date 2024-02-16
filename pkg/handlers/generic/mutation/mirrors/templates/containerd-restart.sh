#!/bin/bash
systemctl restart containerd

if ! command -v crictl; then
  echo "Command crictl is not available, will not wait for Containerd to be running"
  exit
fi

SECONDS=0
until crictl info; do
  if ((SECONDS > 60)); then
    echo "Containerd is not running. Giving up..."
    exit 1
  fi
  echo "Containerd is not running yet. Waiting..."
  sleep 5
done
