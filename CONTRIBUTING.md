<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Contributing

Contributions are welcome. Search the
[issue tracker](https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/issues)
before opening a new issue. To contribute,
[fork](https://docs.github.com/articles/fork-a-repo) this repository, commit
your changes, and open a pull request.

User documentation is published at
<https://nutanix-cloud-native.github.io/cluster-api-runtime-extensions-nutanix/>.

## Development docs

- [Linting](https://nutanix-cloud-native.github.io/cluster-api-runtime-extensions-nutanix/contributing/linting/)
- [API conventions](https://nutanix-cloud-native.github.io/cluster-api-runtime-extensions-nutanix/contributing/api_conventions/)
- [End-to-end tests](https://nutanix-cloud-native.github.io/cluster-api-runtime-extensions-nutanix/contributing/e2e_tests/)
- [Releasing](https://nutanix-cloud-native.github.io/cluster-api-runtime-extensions-nutanix/contributing/releasing/)

Local setup uses [Devbox](https://github.com/jetify-com/devbox) and
[direnv](https://direnv.net/). Common checks:

```shell
make test
make lint
make fmt
```

A change to a topology mutation handler that alters patch output needs a new
handler version. See the Development section of the README.

## Feature requests

Open a feature request in the
[issue tracker](https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/issues)
with the expected behavior and the use case.

## Pull requests

1. A GitHub Action may lint and test the change.
2. A maintainer reviews it. A change needs at least one approving review.
3. The pull request can merge after that approval.

Use the pull request template. Explain how the change was tested.

## Contributor License Agreement

Before a pull request can merge, contributors who are not Nutanix full-time
employees or interns sign the
[Nutanix Contributor License Agreement (CLA)](https://www.nutanix.dev/cla/).
The CLA is substantially similar to the Apache Contributor License Agreement.
