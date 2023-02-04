<!--
 Copyright 2023 D2iQ, Inc. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# CAPI Runtime Extensions Server

See [upstream documentation](https://cluster-api.sigs.k8s.io/tasks/experimental-features/runtime-sdk/index.html).

## Development

To deploy a local build, either initial install to update an existing deployment, run:

```shell
make dev.run-on-kind
eval $(make kind.kubeconfig)
```

To create a cluster with [clusterctl](https://cluster-api.sigs.k8s.io/user/quick-start.html), run:

```shell
clusterctl generate cluster capi-quickstart \
  --flavor development \
  --kubernetes-version v1.26.0 \
  --control-plane-machine-count=1 \
  --worker-machine-count=1 | \
  kubectl apply -f -
```

Label the cluster to deploy Calico:

```shell
kubectl label cluster capi-quickstart capi-runtime-extensions.d2iq-labs.com/cni=calico
```

To get the kubeconfig for the new cluster, run:

```shell
clusterctl get kubeconfig capi-quickstart > capd-kubeconfig
```

If you are not on Linux, you will also need to fix the generated kubeconfig's `server`, run:

```shell
kubectl config set-cluster capi-quickstart \
  --kubeconfig capd-kubeconfig \
  --server=https://$(docker port capi-quickstart-lb 6443/tcp)
```

To delete the workload cluster, run:

```shell
kubectl delete cluster capi-quickstart
```

To delete the dev KinD cluster, run:

```shell
make kind.delete
```
