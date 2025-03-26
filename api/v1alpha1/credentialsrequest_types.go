package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CredentialsMode string

const (
	CredentialsModePassthorugh CredentialsMode = "passthrough"
	CredentialsModeMint        CredentialsMode = "mint"
	LabelRootSecretWatchKey    string          = "caren.nutanix.com/credentials-root"
)

type RootCredentialsKey string

const (
	RootCredentialsKeyPCEndpoint RootCredentialsKey = "pcEndpoint"
)

type Infrastructure string

const (
	InfrastructureNutanix Infrastructure = "nutanix"
)

type Component string

const (
	ComponentNutanixCluster Component = "nutanix-cluster"
)

// CredentialsRequestSpec defines the desired state of CredentialsRequest.
type CredentialsRequestSpec struct {
	SecretRef          corev1.SecretReference `json:"secretRef"`
	ClusterSelector    metav1.LabelSelector   `json:"clusterSelector"`
	RootCredentialsKey RootCredentialsKey     `json:"rootCredentialsKey"`
	Mode               CredentialsMode        `json:"mode"`
	Infrastructure     Infrastructure         `json:"infrastructure"`
	Component          Component              `json:"component"`
	Rotation           RotationSpec           `json:"rotation,omitempty"`
}

type FrequencyType string

const (
	FrequencyTypeOnce  FrequencyType = "once"
	FrequencyTypeEvery FrequencyType = "every"
)

type RotationSpec struct {
	// RotationInterval is the interval at which the credentials should be rotated.
	Interval  metav1.Duration `json:"interval,omitempty"`
	Frequency FrequencyType   `json:"frequency,omitempty"`
}

// CredentialsRequestStatus defines the observed state of CredentialsRequest.
type CredentialsRequestStatus struct {
	LastRotated metav1.Time `json:"lastRotated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CredentialsRequest is the Schema for the credentialsrequests API.
type CredentialsRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CredentialsRequestSpec   `json:"spec,omitempty"`
	Status CredentialsRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CredentialsRequestList contains a list of CredentialsRequest.
type CredentialsRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CredentialsRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CredentialsRequest{}, &CredentialsRequestList{})
}
