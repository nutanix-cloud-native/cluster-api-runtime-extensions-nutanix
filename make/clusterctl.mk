# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

define AWS_CREDENTIALS
[default]
aws_access_key_id = $(AWS_ACCESS_KEY_ID)
aws_secret_access_key = $(AWS_SECRET_ACCESS_KEY)
aws_session_token = $(AWS_SESSION_TOKEN)
endef

AWS_B64ENCODED_CREDENTIALS ?= $(shell echo "$$AWS_CREDENTIALS" | base64 -w0)

.PHONY: clusterctl.init
clusterctl.init:
	env CLUSTER_TOPOLOGY=true \
			EXP_RUNTIME_SDK=true \
			EXP_CLUSTER_RESOURCE_SET=true \
			EXP_MACHINE_POOL=true \
			AWS_B64ENCODED_CREDENTIALS=$(AWS_B64ENCODED_CREDENTIALS) \
			clusterctl init \
		--kubeconfig=$(KIND_KUBECONFIG) \
		--infrastructure docker,aws \
		--wait-providers

.PHONY: clusterctl.delete
clusterctl.delete:
	clusterctl delete --kubeconfig=$(KIND_KUBECONFIG) --all
