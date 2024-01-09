+++
title = "CAPI Runtime Extensions"

# [[cascade]]
# type = "blog"
# toc_root = true

#   [cascade._target]
#   path = "/blog/**"

[[cascade]]
type = "docs"

  [cascade._target]
  path = "/**"
+++

Cluster API provides declarative APIs for provisioning, upgrading, and operating Kubernetes clusters across multiple
infrastructure providers. The [`ClusterClass`][clusterclass] feature brings huge improvements in how users manage
clusters, providing a single resource for a user to mutate to orchestrate upgrades, etc. `ClusterClass` also brings much
improved templating over the [`clusterctl generate cluster`][clusterctl generate cluster] environment-variable driven
templating by introducing variables specified with an OpenAPI schema that can then be applied to the generated resources
via patches.

The [Runtime SDK] feature provides an extensibility mechanism to hook into `ClusterClass` managed Kubernetes clusters'
lifecycle. This project, CAPI Runtime Extensions, provides implementations of various runtime hooks that can be used in
`ClusterClasses` across providers. This includes variables and patches that can be used across any provider to configure
generic Kubernetes capabilities, such as configuring audit policy or HTTP proxy configuration. These capabilities are
not provider-specific and delivering these capabilities in code instead of directly embedded in `ClusterClass`
definitions leads to a much more robust experience via fast-feedback unit tests, as opposed to long running e2e tests.

In addition to cluster resource customizations, this project enables management of essential cluster addons (e.g. CNI)
via variable definitions, e.g. selecting a CNI provider via variables defined on the `Cluster` resource itself. The goal
is to provide a single resource, the `Cluster`, that a user has to interact with to describe a fully-operational
Kubernetes cluster.

[clusterclass]: https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/
[clusterctl generate cluster]: https://cluster-api.sigs.k8s.io/clusterctl/commands/generate-cluster.html
[Runtime SDK]: https://cluster-api.sigs.k8s.io/tasks/experimental-features/runtime-sdk/
