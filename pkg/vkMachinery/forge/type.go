package forge

import (
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// VirtualKubeletOpts defines the custom options associated with the virtual kubelet deployment forging.
type VirtualKubeletOpts struct {
	// ContainerImage contains the virtual kubelet image name and tag.
	ContainerImage string
	// InitContainerImage contains the virtual kubelet init-container image name and tag.
	InitContainerImage string
	// DisableCertGeneration allows to disable the virtual kubelet certificate generation by means
	// of the init container (used for logs/exec capabilities).
	DisableCertGeneration bool
	ExtraAnnotations      map[string]string
	ExtraLabels           map[string]string
	ExtraArgs             []string
	NodeExtraAnnotations  argsutils.StringMap
	NodeExtraLabels       argsutils.StringMap
}
