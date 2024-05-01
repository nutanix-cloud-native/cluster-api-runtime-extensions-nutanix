+++
title = "Deploying CAREN"
icon = "fa-solid fa-truck-fast"
+++

CAREN is implemented as a CAPI runtime extension provider, which means it can be deployed alongside all other CAPI
providers in the same way [using `clusterctl`]({{< ref "via-clusterctl" >}}). However, as CAREN is not yet integrated
into `clusterctl`, it is necessary to first configure `clusterctl` to know about CAREN before we can deploy it.

Alternatively you can install CAREN [via Helm]({{< ref "via-helm" >}}). Installing via Helm will provide default some
default `ClusterClasses` and allow for further customization of the CAREN deployment.
