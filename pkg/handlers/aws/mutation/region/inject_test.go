// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package region

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	regiontests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/region/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestRegionPatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS Region mutator suite")
}

var _ = Describe("Generate AWS Region patches", func() {
	// only add aws region patch
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewPatch()).(mutation.GeneratePatches)
	}
	regiontests.TestGeneratePatches(
		GinkgoT(),
		patchGenerator,
		clusterconfig.MetaVariableName,
		v1alpha1.AWSVariableName,
		VariableName,
	)
})
