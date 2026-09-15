<!--
 Copyright 2023 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# CAPI Runtime Extensions

For user docs, please see [https://nutanix-cloud-native.github.io/cluster-api-runtime-extensions-nutanix/].

See [upstream documentation](https://cluster-api.sigs.k8s.io/tasks/experimental-features/runtime-sdk/index.html).

## Development

### Implementing Topology Mutation Handler

See examples of existing [topology mutation handlers] in the `pkg/handlers/../mutation/` directory.
When adding a new handler, or modifying an existing one, pay close attention to what happens to existing clusters
when a new version of this extension is deployed in the management cluster,
and avoid rollouts of Machines in those existing clusters.

If a change to a handler is unavoidable, you must:

1. Update the version of the handlers, e.g. from `v4` to `v5`.
   This is done by updating the `..ClusterV4ConfigPatch` in the handler files in `pkg/handlers/../mutation/`.
   And `..clusterv4configpatch` in `hack/examples/overlays/clusterclasses/../kustomization.yaml.tmpl`
2. Update the version of the handler in the `pkg/handlers/v3` package, e.g. from `v3` to `v4`.
3. Copy the existing implementation of the handler to `pkg/handlers/../v4/`.

During CAPI provider upgrades, and periodically, all managed clusters are reconciled and mutation handler patches
are applied.
Any new handlers that return a new set of patches, or updated handlers that return a different set of patches,
will be applied causing a rollout of Machines in all managed clusters.

For example, when adding a new handler, a handler that is enabled by default and returns CAPI resources patches,
will cause a rollout of Machines.
Similarly, if a handler is modified to return a different set of patches, it will also cause a rollout of Machines.

### Run Locally

Install tools

- [Devbox](https://github.com/jetpack-io/devbox?tab=readme-ov-file#installing-devbox)
- [Direnv](https://direnv.net/docs/installation.html)
- Container Runtime for your Operating System

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

You can just update the image in the webhook Deployment on an existing KIND cluster:

```shell
make KIND_CLUSTER_NAME=<> dev.update-webhook-image-on-kind
```

Generate a cluster definition from the file specified in the `--from` flag
and apply the generated resource to actually create the cluster in the API.
For example, the following command will create a Docker cluster with Cilium CNI applied via the Helm addon provider:

```shell
export CLUSTER_NAME=docker-cluster-cilium-helm-addon
export CLUSTER_FILE=examples/capi-quick-start/docker-cluster-cilium-helm-addon.yaml
export KUBERNETES_VERSION=v1.35.3
```

```shell
clusterctl generate cluster ${CLUSTER_NAME} \
  --from ${CLUSTER_FILE} \
  --kubernetes-version ${KUBERNETES_VERSION} \
  --worker-machine-count 1 | \
  kubectl apply --server-side -f -
```

Wait until control plane is ready:

```shell
kubectl wait clusters/${CLUSTER_NAME} --for=condition=ControlPlaneInitialized --timeout=5m
```

To get the kubeconfig for the new cluster, run:

```shell
clusterctl get kubeconfig ${CLUSTER_NAME} > ${CLUSTER_NAME}.conf
```

If you are not on Linux, you will also need to fix the generated kubeconfig's `server`, run:

```shell
kubectl config set-cluster ${CLUSTER_NAME} \
  --kubeconfig ${CLUSTER_NAME}.conf \
  --server=https://$(docker container port ${CLUSTER_NAME}-lb 6443/tcp)
```

Wait until all nodes are ready (this indicates that CNI has been deployed successfully):

```shell
kubectl --kubeconfig ${CLUSTER_NAME}.conf wait nodes --all --for=condition=Ready --timeout=5m
```

Show that Cilium is running successfully on the workload cluster:

```shell
kubectl --kubeconfig ${CLUSTER_NAME}.conf get daemonsets -n kube-system cilium
```

Deploy kube-vip to provide service load-balancer functionality for Docker clusters:

```shell
helm repo add --force-update kube-vip https://kube-vip.github.io/helm-charts
helm repo update
kind_subnet_prefix="$(docker network inspect kind -f '{{ (index .IPAM.Config 0).Subnet }}' | \
                      grep -o '^[[:digit:]]\+\.[[:digit:]]\+\.')"
kubectl create configmap \
  --namespace kube-system kubevip \
  --from-literal "range-global=${kind_subnet_prefix}100.0-${kind_subnet_prefix}100.20" \
  --dry-run=client -oyaml |
  kubectl --kubeconfig ${CLUSTER_NAME}.conf apply --server-side -n kube-system -f -

helm upgrade kube-vip-cloud-provider kube-vip/kube-vip-cloud-provider --version 0.2.2 \
  --install \
  --wait --wait-for-jobs \
  --namespace kube-system \
  --kubeconfig ${CLUSTER_NAME}.conf \
  --set-string=image.tag=v0.0.6

helm upgrade kube-vip kube-vip/kube-vip --version 0.4.2 \
  --install \
  --wait --wait-for-jobs \
  --namespace kube-system \
  --kubeconfig ${CLUSTER_NAME}.conf \
  --set-string=image.tag=v0.6.0
```

Deploy traefik as a LB service:

```shell
helm --kubeconfig ${CLUSTER_NAME}.conf repo add traefik https://helm.traefik.io/traefik
helm repo update &>/dev/null
helm --kubeconfig ${CLUSTER_NAME}.conf upgrade --install traefik traefik/traefik \
  --version v10.9.1 \
  --wait --wait-for-jobs \
  --set ports.web.hostPort=80 \
  --set ports.websecure.hostPort=443 \
  --set service.type=LoadBalancer
```

Watch for traefik LB service to get an external address:

```shell
watch -n 0.5 kubectl --kubeconfig ${CLUSTER_NAME}.conf get service/traefik
```

To delete the workload cluster, run:

```shell
kubectl delete cluster ${CLUSTER_NAME}
```

Notice that the traefik service is deleted before the cluster is actually finally deleted.

Check the pod logs:

```shell
kubectl logs deployment/cluster-api-runtime-extensions-nutanix -f
```

To delete the dev KinD cluster, run:

```shell
make kind.delete
```

[topology mutation handlers]: https://cluster-api.sigs.k8s.io/tasks/experimental-features/runtime-sdk/implement-topology-mutation-hook#implementing-topology-mutation-hook-runtime-extensions

## Update Addons

Addon and Helm chart versions are defined in [`make/addons.mk`](make/addons.mk). That file is the source of truth: each addon version is represented by an environment variable, e.g. `CILIUM_VERSION`.

To update an addon, edit its version variable in `make/addons.mk`, then run:

```shell
make addons.sync
```

Then, commit the changes.

### How Addons Sync Works

`make addons.sync` runs two largely independent stages. Understanding both explains why some addons have
`hack/addons/update-*.sh` scripts and others (like the Nutanix CSI driver) do not.

#### 1. Vendoring scripts (CRS / static manifests)

The first stage runs only the targets listed in [`make/addons.mk`](make/addons.mk) under `addons.sync`:

- Calico (Tigera operator chart rendered into static YAML)
- Cilium, Node Feature Discovery, cluster-autoscaler, snapshot-controller
- AWS EBS CSI, local-path-provisioner CSI
- kube-vip
- AWS cloud-controller-manager (one run per supported Kubernetes minor, e.g. 1.30–1.34)

Each of these invokes a script under `hack/addons/` that:

- Reads version variables exported from `addons.mk` (e.g. `CILIUM_VERSION`, `AWS_EBS_CSI_CHART_VERSION`).
- Often builds a temporary kustomization from `hack/addons/kustomize/<addon>/` templates.
- Fetches or renders upstream manifests and writes checked-in files under
  `charts/cluster-api-runtime-extensions-nutanix/…` (CRS-style addon payloads, kube-vip static YAML, etc.).

Addons that are only installed via the Helm addon provider—chart pulled at cluster reconcile time with no
pre-vendored manifest directory in the chart—do not need this step. Examples include Nutanix Storage CSI,
Nutanix CCM, MetalLB, AWS Load Balancer Controller, COSI controller, and konnector-agent. For
those, the kustomization template uses a variable such as `${NUTANIX_STORAGE_CSI_CHART_VERSION}`; bumping it in
`addons.mk` and running sync refreshes `helm-config.yaml`. There is no `update-nutanix-storage-csi.sh` because nothing
is vendored into the chart from that upstream chart. A few other Helm addons under `hack/addons/kustomize/` (e.g.
multus, registry-syncer, CNCF distribution registry) still pin literal chart versions in their templates—see the
caveat below.

kube-vip is the opposite edge case: it has a vendoring script but no `helmCharts` entry under
`hack/addons/kustomize/`, so it does not appear in the generated `helm-config.yaml`—it is static YAML only.

#### 2. Helm addon ConfigMap (`helm-config.yaml`)

After the scripts finish, `addons.sync` runs `template-helm-repository`, which (via `generate-mindthegap-repofile`)
runs [`hack/tools/helm-cm`](hack/tools/helm-cm/main.go). That tool walks every
`hack/addons/kustomize/**/kustomization.yaml.tmpl` that contains a `helmCharts` block. For each directory it records
repository URL, chart name, and chart version into
`charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml`. Version strings in the template can use
shell-style expansion (e.g. `version: ${NUTANIX_STORAGE_CSI_CHART_VERSION}`), which Make resolves from `addons.mk`
when the tool runs.

So for Helm-only addons, the flow is: edit the variable in `addons.mk` → `make addons.sync` → commit the updated
`helm-config.yaml` (and mindthegap repofile artifacts if present). No separate update script.

Caveat: If a `kustomization.yaml.tmpl` uses a literal chart version (not `${…}`), changing the version requires
editing that template file; `addons.mk` alone will not update that chart’s entry until the literal is changed.

#### Summary

| Kind of addon | Example | Vendoring script on `addons.sync` | Version in `helm-config` derived from |
|---------------|---------|-----------------------------------|-------------------------------|
| CRS + Helm metadata | Cilium, AWS EBS CSI | Yes | `addons.mk` + kustomization |
| Helm only | Nutanix Storage CSI, MetalLB | No | `addons.mk` + kustomization (`${VAR}`) |
| Static YAML only | kube-vip | Yes | N/A (not in `helm-config`) |
| Literal version in tmpl | (e.g. multus chart) | No | Edit tmpl or add `addons.mk` var + tmpl reference |
