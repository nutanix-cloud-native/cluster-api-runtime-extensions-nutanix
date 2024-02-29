+++
title = "LoadBalancer Services Garbage Collection"
icon = "fa-solid fa-recycle"
+++

When using Kubernetes `LoadBalancer` services, the relevant cloud provider interface creates and configures external
resources. If the `LoadBalancer` services are not deleted prior to deleting the Kubernetes cluster, then these external
resources are orphaned, leading to wasted resources and unnecessary expense. The load-balancer services garbage
collector is implemented as a `BeforeClusterDelete` CAPI cluster lifecycle hook that deletes the load-balancer services
and thus triggering the cloud provider interface to clean up the external resources. The hook blocks until all
load-balancer services have been fully deleted, indicating that the cloud provider interface has cleaned up the external
resources.

By default, all clusters will be cleaned up when deleting, but this can be opted out from by setting the annotation
`capiext.labs.d2iq.io/loadbalancer-gc=false`.
