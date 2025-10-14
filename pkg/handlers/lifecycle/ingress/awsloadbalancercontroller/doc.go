// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package awsloadbalancercontroller provides lifecycle handlers for deploying the AWS Load Balancer Controller addon.
//
// The AWS Load Balancer Controller manages AWS Application Load Balancers (ALB) and Network Load Balancers (NLB)
// for Kubernetes services and ingresses. This package provides handlers that deploy the controller using
// the Cluster API Add-on Provider for Helm (CAAPH).
//
// The handler automatically installs the AWS Load Balancer Controller during the AfterControlPlaneInitialized
// lifecycle phase, ensuring the controller is available for managing load balancer resources.
package awsloadbalancercontroller
