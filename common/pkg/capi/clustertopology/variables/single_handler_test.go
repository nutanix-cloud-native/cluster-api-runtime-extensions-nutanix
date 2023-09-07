package variables

import (
	"testing"

	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
)

func TestSingleVariableHandler(t *testing.T) {
	vars := Group{
		"foo": inlineVariable{v: clusterv1.VariableSchema{
			OpenAPIV3Schema: clusterv1.JSONSchemaProps{
				Description: "foo desc",
				Type:        "string",
			},
		}},
		"bar": inlineVariable{v: clusterv1.VariableSchema{
			OpenAPIV3Schema: clusterv1.JSONSchemaProps{
				Description: "bar desc",
				Type:        "string",
			},
		}},
	}

	capitest.ValidateDiscoverVariables(
		t,
		"single",
		ptr.To(vars.VariableSchema()),
		func() *singleVariableHandler {
			return NewAsSingleVariableHandler("single", vars, false)
		},
		capitest.VariableTestDef{
			Name: "valid values",
			Vals: map[string]any{
				"foo": "fooval",
				"bar": "barval",
			},
		},
	)
}
