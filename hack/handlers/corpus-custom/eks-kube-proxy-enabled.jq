# Copyright 2026 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# Leaves kube-proxy enabled in the control plane template, so only the kubeProxy variable can disable it.
if .kind == "AWSManagedControlPlaneTemplate" then
  del(.spec.template.spec.kubeProxy)
else . end
