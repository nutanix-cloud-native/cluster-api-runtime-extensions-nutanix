#!/bin/bash
set -euo pipefail

# This script is used to discover the placement group for the node.
# The script will be placed in /etc/kubernetes/node-feature-discovery/source.d/
# The script will be executed by the node feature discovery's local feature hook.
PLACEMENT_GROUP_FEATURE_DIR="/etc/kubernetes/node-feature-discovery/features.d"
PLACEMENT_GROUP_FEATURE_FILE="${PLACEMENT_GROUP_FEATURE_DIR}/placementgroup"
# Fetch IMDSv2 token
TOKEN=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" \
  -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")

# Get placement info with HTTP status check
PARTITION_RESPONSE=$(curl -s -w "%{http_code}" -H "X-aws-ec2-metadata-token: $TOKEN" \
  http://169.254.169.254/latest/meta-data/placement/partition-number)

PG_RESPONSE=$(curl -s -w "%{http_code}" -H "X-aws-ec2-metadata-token: $TOKEN" \
  http://169.254.169.254/latest/meta-data/placement/group-name)

# Extract HTTP status codes and content
PARTITION_HTTP_CODE="${PARTITION_RESPONSE: -3}" # last 3 characters are the HTTP status code
PARTITION_CONTENT="${PARTITION_RESPONSE%???}"   # remove the last 3 characters to get the content

mkdir -p "${PLACEMENT_GROUP_FEATURE_DIR}"
touch "${PLACEMENT_GROUP_FEATURE_FILE}"

# Only print features if HTTP 200 response
if [ "$PARTITION_HTTP_CODE" = "200" ] && [ -n "$PARTITION_CONTENT" ]; then
  echo "feature.node.kubernetes.io/partition=${PARTITION_CONTENT}" >>"${PLACEMENT_GROUP_FEATURE_FILE}"
fi

PG_HTTP_CODE="${PG_RESPONSE: -3}" # last 3 characters are the HTTP status code
PG_CONTENT="${PG_RESPONSE%???}"   # remove the last 3 characters to get the content

if [ "$PG_HTTP_CODE" = "200" ] && [ -n "$PG_CONTENT" ]; then
  echo "feature.node.kubernetes.io/aws-placement-group=${PG_CONTENT}" >>"${PLACEMENT_GROUP_FEATURE_FILE}"
fi
