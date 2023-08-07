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

Pro-tip: to redeploy without rebuilding the binaries, images, etc (useful if you have only changed the Helm chart for
example), run:

```shell
make SKIP_BUILD=true dev.run-on-kind
```

To create a cluster with [clusterctl](https://cluster-api.sigs.k8s.io/user/quick-start.html), run:

```shell
env POD_SECURITY_STANDARD_ENABLED=false \
  clusterctl generate cluster capi-quickstart \
    --flavor development \
    --kubernetes-version v1.27.2 \
    --control-plane-machine-count=1 \
    --worker-machine-count=1 | \
  kubectl apply --server-side -f -
```

Wait until control plane is ready:

```shell
kubectl wait clusters/capi-quickstart --for=condition=ControlPlaneInitialized --timeout=5m
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

Deploy Calico to the workload cluster (TODO deploy via lifecycle hook):

```shell
helm repo add --force-update projectcalico https://docs.tigera.io/calico/charts
helm upgrade --install calico projectcalico/tigera-operator \
  --version v3.26.1 \
  --namespace tigera-operator \
  --create-namespace \
  --wait \
  --wait-for-jobs \
  --kubeconfig capd-kubeconfig
```

Wait until all nodes are ready (this indicates that CNI has been deployed successfully):

```shell
kubectl --kubeconfig capd-kubeconfig wait nodes --all --for=condition=Ready --timeout=5m
```

Show that Calico is running successfully on the workload cluster:

```shell
kubectl --kubeconfig capd-kubeconfig get daemonsets -n calico-system
```

Deploy kube-vip to provide service load-balancer:

```shell
helm repo add --force-update kube-vip https://kube-vip.github.io/helm-charts
helm repo update
kind_subnet_prefix="$(docker network inspect kind -f '{{ (index .IPAM.Config 0).Subnet }}' | \
                      grep -o '^[[:digit:]]\+\.[[:digit:]]\+\.')"
kubectl create configmap \
  --namespace kube-system kubevip \
  --from-literal "range-global=${kind_subnet_prefix}100.0-${kind_subnet_prefix}100.20" \
  --dry-run=client -oyaml |
  kubectl --kubeconfig capd-kubeconfig apply --server-side -n kube-system -f -

helm upgrade kube-vip-cloud-provider kube-vip/kube-vip-cloud-provider --version 0.2.2 \
  --install \
  --wait --wait-for-jobs \
  --namespace kube-system \
  --kubeconfig capd-kubeconfig \
  --set-string=image.tag=v0.0.6

helm upgrade kube-vip kube-vip/kube-vip --version 0.4.2 \
  --install \
  --wait --wait-for-jobs \
  --namespace kube-system \
  --kubeconfig capd-kubeconfig \
  --set-string=image.tag=v0.6.0
```

Deploy traefik as a LB service:

```shell
helm --kubeconfig capd-kubeconfig repo add traefik https://helm.traefik.io/traefik
helm repo update &>/dev/null
helm --kubeconfig capd-kubeconfig upgrade --install traefik traefik/traefik \
  --version v10.9.1 \
  --wait --wait-for-jobs \
  --set ports.web.hostPort=80 \
  --set ports.websecure.hostPort=443 \
  --set service.type=LoadBalancer
```

Watch for traefik LB service to get an external address:

```shell
watch -n 0.5 kubectl --kubeconfig capd-kubeconfig get service/traefik
```

To delete the workload cluster, run:

```shell
kubectl delete cluster capi-quickstart
```

Notice that the traefik service is deleted before the cluster is actually finally deleted.

Check the pod logs:

```shell
kubectl logs deployment/capi-runtime-extensions -f
```

To delete the dev KinD cluster, run:

```shell
make kind.delete
```
