// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/httpproxy"
)

func TestDiscoverVariables(t *testing.T) {
	g := NewWithT(t)
	h := httpproxy.NewVariable()
	resp := &runtimehooksv1.DiscoverVariablesResponse{}
	h.DiscoverVariables(context.Background(), &runtimehooksv1.DiscoverVariablesRequest{}, resp)

	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
	g.Expect(resp.Variables).To(HaveLen(1))

	variable := resp.Variables[0]
	g.Expect(variable).To(MatchFields(IgnoreExtras, Fields{
		"Name":     Equal(httpproxy.VariableName),
		"Required": BeFalse(),
		"Schema":   Equal(httpproxy.HTTPProxyVariables{}.VariableSchema()),
	}))
}
