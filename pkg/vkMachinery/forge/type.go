package forge

// VirtualKubeletOpts defines the custom options associated with the virtual kubelet deployment forging.
type VirtualKubeletOpts struct {
	// ContainerImage contains the virtual kubelet image name and tag.
	ContainerImage string
	// InitContainerImage contains the virtual kubelet init-container image name and tag.
	InitContainerImage string
	// DisableCertGeneration allows to disable the virtual kubelet certificate generation by means
	// of the init container (used for logs/exec capabilities).
	DisableCertGeneration bool
}
