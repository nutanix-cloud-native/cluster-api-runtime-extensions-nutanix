<!--
 Copyright 2024 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Test Framework

## Origin

This directory is a copy, with modifications, of the [upstream test
package](https://github.com/kubernetes-sigs/cluster-api/tree/v1.7.2/internal/test).

## Purpose

The namespacesync controller reads and writes Templates of various types. The
upstream test package creates "generic" Template types and CRDs. This allows us
to test the controller using envtest, with real types and CRDs.
