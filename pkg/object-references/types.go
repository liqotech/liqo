package object_references

// DeploymentReference represents a Deployment Reference. It has enough information to retrieve deployment
// in any namespace
type DeploymentReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}
