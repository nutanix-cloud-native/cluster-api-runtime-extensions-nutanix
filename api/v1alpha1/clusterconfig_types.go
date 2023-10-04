// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/openapi/patterns"
)

const (
	CNIProviderCalico                 = "calico"
	CSIProviderAWSEBS                 = "aws-ebs-sc"
	CSIProviderLocalVolumeProvisioner = "localvolumeprovisioner"
)

//+kubebuilder:object:root=true

// ClusterConfig is the Schema for the clusterconfigs API.
type ClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	//+optional
	Spec ClusterConfigSpec `json:"spec,omitempty"`
}

// ClusterConfigSpec defines the desired state of ClusterConfig.
type ClusterConfigSpec struct {
	// +optional
	AWS *AWSSpec `json:"aws,omitempty"`
	// +optional
	Docker *DockerSpec `json:"docker,omitempty"`

	GenericClusterConfig `json:",inline"`
}

func (s ClusterConfigSpec) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	clusterConfigProps := GenericClusterConfig{}.VariableSchema()

	switch {
	case s.AWS != nil:
		maps.Copy(
			clusterConfigProps.OpenAPIV3Schema.Properties,
			map[string]clusterv1.JSONSchemaProps{
				"aws": AWSSpec{}.VariableSchema().OpenAPIV3Schema,
			},
		)

		clusterConfigProps.OpenAPIV3Schema.Required = append(
			clusterConfigProps.OpenAPIV3Schema.Required,
			"aws",
		)
	case s.Docker != nil:
		maps.Copy(
			clusterConfigProps.OpenAPIV3Schema.Properties,
			map[string]clusterv1.JSONSchemaProps{
				"docker": DockerSpec{}.VariableSchema().OpenAPIV3Schema,
			},
		)

		clusterConfigProps.OpenAPIV3Schema.Required = append(
			clusterConfigProps.OpenAPIV3Schema.Required,
			"docker",
		)
	}

	return clusterConfigProps
}

// GenericClusterConfig defines the generic cluster configdesired.
type GenericClusterConfig struct {
	// +optional
	KubernetesImageRepository *KubernetesImageRepository `json:"kubernetesImageRepository,omitempty"`

	// +optional
	Etcd *Etcd `json:"etcd,omitempty"`

	// +optional
	Proxy *HTTPProxy `json:"proxy,omitempty"`

	// +optional
	ExtraAPIServerCertSANs ExtraAPIServerCertSANs `json:"extraAPIServerCertSANs,omitempty"`

	// +optional
	ImageRegistries ImageRegistries `json:"imageRegistries,omitempty"`

	// +optional
	Addons *Addons `json:"addons,omitempty"`
}

func (GenericClusterConfig) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"addons":                 Addons{}.VariableSchema().OpenAPIV3Schema,
				"etcd":                   Etcd{}.VariableSchema().OpenAPIV3Schema,
				"extraAPIServerCertSANs": ExtraAPIServerCertSANs{}.VariableSchema().OpenAPIV3Schema,
				"proxy":                  HTTPProxy{}.VariableSchema().OpenAPIV3Schema,
				"kubernetesImageRepository": KubernetesImageRepository(
					"",
				).VariableSchema().
					OpenAPIV3Schema,
				"imageRegistries": ImageRegistries{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

// KubernetesImageRepository required for overriding Kubernetes image repository.
type KubernetesImageRepository string

func (KubernetesImageRepository) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Sets the Kubernetes image repository used for the KubeadmControlPlane.",
			Type:        "string",
			Pattern:     patterns.Anchored(patterns.ImageRepository),
		},
	}
}

func (v KubernetesImageRepository) String() string {
	return string(v)
}

type Image struct {
	// Repository is used to override the image repository to pull from.
	//+optional
	Repository string `json:"repository,omitempty"`

	// Tag is used to override the default image tag.
	//+optional
	Tag string `json:"tag,omitempty"`
}

func (Image) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"repository": {
					Description: "Image repository to pull from.",
					Type:        "string",
					Pattern:     patterns.Anchored(patterns.ImageRepository),
				},
				"tag": {
					Description: "Image tag to use.",
					Type:        "string",
					Pattern:     patterns.Anchored(patterns.ImageTag),
				},
			},
		},
	}
}

type Etcd struct {
	// Image required for overriding etcd image details.
	//+optional
	Image *Image `json:"image,omitempty"`
}

func (Etcd) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"image": Image{}.VariableSchema().OpenAPIV3Schema,
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

type ImageRegistries struct {
	// +optional
	ImageRegistryCredentials ImageRegistryCredentials `json:"credentials,omitempty"`
}

func (ImageRegistries) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Configuration for image registries.",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"credentials": ImageRegistryCredentials{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

type ImageRegistryCredentials []ImageRegistryCredentialsResource

func (ImageRegistryCredentials) VariableSchema() clusterv1.VariableSchema {
	resourceSchema := ImageRegistryCredentialsResource{}.VariableSchema().OpenAPIV3Schema

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Image registry credentials to set up on all Nodes in the cluster. " +
				"Enabling this will configure the Kubelets with " +
				"https://kubernetes.io/docs/tasks/administer-cluster/kubelet-credential-provider/.",
			Type:  "array",
			Items: &resourceSchema,
		},
	}
}

// ImageRegistryCredentialsResource required for providing credentials for an image registry URL.
type ImageRegistryCredentialsResource struct {
	// Registry URL.
	URL string `json:"url"`

	// The Secret containing the registry credentials.
	// The Secret should have keys 'username' and 'password'.
	// This credentials Secret is not required for some registries, e.g. ECR.
	// +optional
	Secret *corev1.ObjectReference `json:"secretRef,omitempty"`
}

func (ImageRegistryCredentialsResource) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"url": {
					Description: "Registry URL.",
					Type:        "string",
				},
				"secretRef": {
					Description: "The Secret containing the registry credentials. " +
						"The Secret should have keys 'username' and 'password'. " +
						"This credentials Secret is not required for some registries, e.g. ECR.",
					Type: "object",
					Properties: map[string]clusterv1.JSONSchemaProps{
						"name": {
							Description: "The name of the Secret containing the registry credentials.",
							Type:        "string",
						},
						"namespace": {
							Description: "The namespace of the Secret containing the registry credentials. " +
								"Defaults to the namespace of the KubeadmControlPlaneTemplate and KubeadmConfigTemplate" +
								" that reference this variable.",
							Type: "string",
						},
					},
				},
			},
			Required: []string{"url"},
		},
	}
}

type Addons struct {
	// +optional
	CNI *CNI `json:"cni,omitempty"`

	// +optional
	NFD *NFD `json:"nfd,omitempty"`

	// +optional
	CSIProviders CSIProviders `json:"csiProviders,omitempty"`
}

func (Addons) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"cni": CNI{}.VariableSchema().OpenAPIV3Schema,
				"nfd": NFD{}.VariableSchema().OpenAPIV3Schema,
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

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"provider": {
					Description: "CNI provider to deploy",
					Type:        "string",
					Enum:        variables.MustMarshalValuesToEnumJSON(supportedCNIProviders...),
				},
			},
			Required: []string{"provider"},
		},
	}
}

// NFD tells us to enable or disable the node feature discovery addon.
type NFD struct{}

func (NFD) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
		},
	}
}

type CSIProviders struct {
	// +optional
	Providers []*CSIProvider `json:"providers,omitempty"`
	// +optional
	DefaultClassName string `json:"defualtClassName,omitempty"`
}

type CSIProvider struct {
	Name string `json:"name,omitempty"`
}

func (CSIProviders) VariableSchema() clusterv1.VariableSchema {
	supportedCSIProviders := []string{CSIProviderAWSEBS, CSIProviderLocalVolumeProvisioner}
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"providers": {
					Type: "array",
					Items: &clusterv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]clusterv1.JSONSchemaProps{
							"name": {
								Type: "string",
								Enum: variables.MustMarshalValuesToEnumJSON(
									supportedCSIProviders...),
							},
						},
					},
				},
				"defaultClassName": {
					Type: "string",
					Enum: variables.MustMarshalValuesToEnumJSON(supportedCSIProviders...),
				},
			},
		},
	}
}

func init() {
	SchemeBuilder.Register(&ClusterConfig{})
}
