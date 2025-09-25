+++
title = "Containerd metrics"
+++

Containerd exports metrics to a Prometheus endpoint. The metrics cover
containerd itself, its plugins, e.g. CRI, and information about the
containers managed by containerd.

There are currently no configuration options for metrics, and this
customization will be automatically applied when the [provider-specific
cluster configuration patch]({{< ref ".." >}}) is included in the
`ClusterClass`.
