# Copyright 2026 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# Puts extra args out of order, as a hand-written ClusterClass might.
if .kind == "KubeadmControlPlaneTemplate" then
  .spec.template.spec.kubeadmConfigSpec.clusterConfiguration.apiServer.extraArgs |= reverse
elif .kind == "KubeadmConfigTemplate" then
  .spec.template.spec.joinConfiguration.nodeRegistration.kubeletExtraArgs |= [{name: "v", value: "2"}] + .
else . end
