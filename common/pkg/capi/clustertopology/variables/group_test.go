package variables

import (
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type inlineVariable struct {
	v clusterv1.VariableSchema
}

func (i inlineVariable) VariableSchema() clusterv1.VariableSchema {
	return i.v
}

func TestGroup(t *testing.T) {
	g := NewWithT(t)
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
	g.Expect(vars.VariableSchema()).To(MatchFields(IgnoreExtras, Fields{
		"OpenAPIV3Schema": Equal(clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"foo": vars["foo"].VariableSchema().OpenAPIV3Schema,
				"bar": vars["bar"].VariableSchema().OpenAPIV3Schema,
			},
		}),
	}))
}
