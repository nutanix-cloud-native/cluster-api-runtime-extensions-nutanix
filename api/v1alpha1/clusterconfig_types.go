// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/openapi/patterns"
)

const (
	CNIProviderCalico = "calico"
)

//+kubebuilder:object:root=true

// ClusterConfig is the Schema for the clusterconfigs API.
type ClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterConfigSpec `json:"spec,omitempty"`
}

// ClusterConfigSpec defines the desired state of ClusterConfig.
type ClusterConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	KubernetesImageRegistry *KubernetesImageRegistry `json:"kubernetesImageRegistry,omitempty"`

	// +optional
	Etcd *Etcd `json:"etcd,omitempty"`

	// +optional
	Proxy *HTTPProxy `json:"proxy,omitempty"`

	// +optional
	ExtraAPIServerCertSANs ExtraAPIServerCertSANs `json:"extraAPIServerCertSANs,omitempty"`

	// +optional
	CNI *CNI `json:"cni,omitempty"`
}

func (ClusterConfigSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"kubernetesImageRegistry": KubernetesImageRegistry(
					"",
				).VariableSchema().
					OpenAPIV3Schema,
				"etcd":                   Etcd{}.VariableSchema().OpenAPIV3Schema,
				"proxy":                  HTTPProxy{}.VariableSchema().OpenAPIV3Schema,
				"extraAPIServerCertSANs": ExtraAPIServerCertSANs{}.VariableSchema().OpenAPIV3Schema,
				"cni":                    CNI{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

// KubernetesImageRegistry required for overriding Kubernetes image registry.
type KubernetesImageRegistry string

func (KubernetesImageRegistry) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Sets the Kubernetes image registry used for the KubeadmControlPlane.",
			Type:        "string",
		},
	}
}

func (v KubernetesImageRegistry) String() string {
	return string(v)
}

type Etcd struct {
	// ImageRepository required for overriding etcd image repository.
	ImageRepository string `json:"imageRepository,omitempty"`

	// ImageTag required for overriding etcd image tag.
	ImageTag string `json:"imageTag,omitempty"`
}

func (Etcd) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"imageRepository": {
					Description: "Image repository for etcd.",
					Type:        "string",
				},
				"imageTag": {
					Description: "Image tag for etcd.",
					Type:        "string",
				},
			},
		},
	}
}

// HTTPProxy required for providing proxy configuration.
type HTTPProxy struct {
	// HTTP proxy.
	HTTP string `json:"http,omitempty"`

	// HTTPS proxy.
	HTTPS string `json:"https,omitempty"`

	// AdditionalNo Proxy list that will be added to the automatically calculated
	// values that will apply no_proxy configuration for cluster internal network.
	// Default values: localhost,127.0.0.1,<POD_NETWORK>,<SERVICE_NETWORK>,kubernetes
	//   ,kubernetes.default,.svc,.svc.<SERVICE_DOMAIN>
	AdditionalNo []string `json:"additionalNo"`
}

func (HTTPProxy) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"http": {
					Description: "HTTP proxy value.",
					Type:        "string",
				},
				"https": {
					Description: "HTTPS proxy value.",
					Type:        "string",
				},
				"additionalNo": {
					Description: "Additional No Proxy list that will be added to the automatically calculated " +
						"values required for cluster internal network. " +
						"Default value: localhost,127.0.0.1,<POD_NETWORK>,<SERVICE_NETWORK>,kubernetes," +
						"kubernetes.default,.svc,.svc.<SERVICE_DOMAIN>",
					Type: "array",
					Items: &clusterv1.JSONSchemaProps{
						Type: "string",
					},
				},
			},
		},
	}
}

// ExtraAPIServerCertSANs required for providing API server cert SANs.
type ExtraAPIServerCertSANs []string

func (ExtraAPIServerCertSANs) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Extra Subject Alternative Names for the API Server signing cert",
			Type:        "array",
			UniqueItems: true,
			Items: &clusterv1.JSONSchemaProps{
				Type:    "string",
				Pattern: patterns.Anchored(patterns.DNS1123Subdomain),
			},
		},
	}
}

// CNI required for providing CNI configuration.
type CNI struct {
	Provider string `json:"provider,omitempty"`
}

func (CNI) VariableSchema() clusterv1.VariableSchema {
	supportedCNIProviders := []string{CNIProviderCalico}

	cniProviderEnumVals, err := variables.ValuesToEnumJSON(supportedCNIProviders...)
	if err != nil {
		panic(err)
	}

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"provider": {
					Description: "CNI provider to deploy",
					Type:        "string",
					Enum:        cniProviderEnumVals,
				},
			},
		},
	}
}

// +kubebuilder:object:root=true
func init() {
	SchemeBuilder.Register(&ClusterConfig{})
}
