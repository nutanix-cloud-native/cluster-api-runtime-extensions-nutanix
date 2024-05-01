+++
title = "End-to-end tests"
icon = "fa-solid fa-bugs"
+++

This project uses [Ginkgo] to run end-to-end tests. Tests are labelled to make running targeted or specific tests
easier.

To determine what labels are available, run:

```shell
ginkgo --dry-run -v -tags e2e ./test/e2e
```

Tests are currently labelled via infrastructure provider, CNI provider, and addon strategy. Here are some examples to
specify what tests to run.

Run all AWS tests:

```shell
make e2e-test E2E_LABEL='provider:AWS'
```

Run all Cilium tests:

```shell
make e2e-test E2E_LABEL='cni:Cilium'
```

Labels can also be combined.

Run Cilium tests on AWS:

```shell
make e2e-test E2E_LABEL='provider:AWS && cni:Cilium'
```

To make debugging easier, you can retain the e2e test environment, which by default is cleaned up after tests run:

```shell
make e2e-test E2E_LABEL='provider:AWS && cni:Cilium' E2E_SKIP_CLEANUP=true
```

To speed up the development process, if you have only change e2e tests then you can skip rebuilding the whole project:

```shell
make e2e-test E2E_LABEL='provider:AWS && cni:Cilium' SKIP_BUILD=true
```

[Ginkgo]: https://onsi.github.io/ginkgo/
