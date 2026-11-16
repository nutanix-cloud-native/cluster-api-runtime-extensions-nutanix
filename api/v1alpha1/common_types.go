// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// ObjectMeta is metadata that all persisted resources must have, which includes all objects
// users must create. This is a copy of customizable fields from metav1.ObjectMeta.
//
// For more details on why this is included instead of using metav1.ObjectMeta directly, see
// https://github.com/kubernetes-sigs/cluster-api/blob/v1.3.3/api/v1beta1/common_types.go#L175-L195.
type ObjectMeta struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ControlPlaneEndpointSpec is where the API server is reached.
//
// The address is either given as host, or reserved from a Nutanix subnet named
// by virtualIP.configuration.ipReservation. The rule requires at least one and
// deliberately allows both: once an address has been reserved it is written
// back into host, because that is the field the CNI and the CCM read.
// +kubebuilder:validation:XValidation:rule="has(self.host) || (has(self.virtualIP) && has(self.virtualIP.configuration) && has(self.virtualIP.configuration.ipReservation))",message="either host or virtualIP.configuration.ipReservation must be set"
type ControlPlaneEndpointSpec struct {
	// The hostname on which the API server is serving.
	// Optional when virtualIP.configuration.ipReservation names a subnet to
	// reserve the address from; it is then filled in with the reserved address.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Host string `json:"host,omitempty"`

	// The port on which the API server is serving.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Configuration for the virtual IP provider.
	// +kubebuilder:validation:Optional
	VirtualIPSpec *ControlPlaneVirtualIPSpec `json:"virtualIP,omitempty"`
}

type ControlPlaneVirtualIPSpec struct {
	// Virtual IP provider to deploy.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=KubeVIP
	// +kubebuilder:default=KubeVIP
	Provider string `json:"provider,omitempty"`

	// Configuration for the chosen control-plane virtual IP provider.
	// +kubebuilder:validation:Optional
	Configuration *ControlPlaneVirtualIPConfiguration `json:"configuration,omitempty"`
}

type ControlPlaneVirtualIPConfiguration struct {
	// The virtual IP on which the API server is serving.
	// If left empty, the value from controlPlaneEndpoint.host will be used.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=ipv4
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address,omitempty"`

	// The port on which the API server is serving.
	// If left empty, the value from controlPlaneEndpoint.port will be used.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// IPReservation reserves the API server address from a Nutanix subnet
	// instead of requiring the operator to pick a free address themselves.
	// +kubebuilder:validation:Optional
	IPReservation *ControlPlaneIPReservation `json:"ipReservation,omitempty"`
}

// ControlPlaneIPReservation reserves one address for the API server from a
// Nutanix subnet, so the operator does not have to find a free one. The subnet
// is identified the same way as everywhere else in this API, which is also how
// NutanixIPPool identifies one.
type ControlPlaneIPReservation struct {
	// Subnet is the Nutanix subnet the API server address is reserved from, as
	// either a UUID or the name of a subnet. The subnet must be IPAM-managed.
	// When a name is used, cluster must identify the Prism Element cluster that
	// resolves it.
	// Immutable: the address is reserved once, and changing the subnet would
	// orphan that reservation while the API server still answers on it.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value cannot be changed after cluster creation"
	Subnet string `json:"subnet"`

	// Cluster is the Prism Element cluster used to resolve a subnet name, as
	// either a name or a UUID. Required only when subnet is a name.
	// Immutable, for the same reason as subnet.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value cannot be changed after cluster creation"
	Cluster *string `json:"cluster,omitempty"`
}

// LocalObjectReference contains enough information to let you locate the
// referenced object inside the same namespace.
type LocalObjectReference struct {
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

func (s ControlPlaneEndpointSpec) VirtualIPAddress() string {
	// If specified, use the virtual IP address and/or port,
	// otherwise fall back to the control plane endpoint host and port.
	if s.VirtualIPSpec != nil &&
		s.VirtualIPSpec.Configuration != nil &&
		s.VirtualIPSpec.Configuration.Address != "" {
		return s.VirtualIPSpec.Configuration.Address
	}

	return s.Host
}
