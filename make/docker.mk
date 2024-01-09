# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: dockerauth
dockerauth:
ifdef DOCKER_HUB_USERNAME
ifdef DOCKER_HUB_PASSWORD
	echo -n $(DOCKER_HUB_PASSWORD) | docker login -u $(DOCKER_HUB_USERNAME) --password-stdin
endif
endif
