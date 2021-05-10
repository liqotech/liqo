package object_references

// DeploymentReference represents a Deployment Reference. It has enough information to retrieve deployment
// in any namespace.
type DeploymentReference struct {
	// Name is unique within a namespace to reference a deployment resource.
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Namespace defines the space within which the deployment name must be unique.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
}

// NodeReference represents a Node Reference. It has enough information to retrieve a node.
type NodeReference struct {
	// Name is unique to reference a node resource.
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
}
