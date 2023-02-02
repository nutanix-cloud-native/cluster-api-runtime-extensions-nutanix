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
```

To delete the dev KinD cluster, run:

```shell
make kind.delete
```
