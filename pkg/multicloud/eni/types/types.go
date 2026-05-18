package types

// MultiCloudSpec is the multicloud IPAM specific configuration of a node,
// populated by the cilium-agent at startup via cloud metadata detection.
type Spec struct {
	// CloudProvider is the detected cloud provider (tencentcloud, aws, alibabacloud).
	//
	// +kubebuilder:validation:Optional
	CloudProvider string `json:"cloud-provider,omitempty"`

	// Region is the cloud region where the node resides.
	//
	// +kubebuilder:validation:Optional
	Region string `json:"region,omitempty"`

	// VPCID is the VPC ID of the node's primary network interface.
	//
	// +kubebuilder:validation:Optional
	VPCID string `json:"vpc-id,omitempty"`

	// SubnetID is the subnet ID of the node's primary network interface (eth0).
	// For Alibaba Cloud this corresponds to the vswitch-id.
	//
	// +kubebuilder:validation:Optional
	SubnetID string `json:"subnet-id,omitempty"`
}
