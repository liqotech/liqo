package forge

import (
	discoveryv1 "k8s.io/api/discovery/v1"
)

func (f *apiForger) endpointsliceHomeToForeign(homeEndpointslice, foreignEndpointslice *discoveryv1.EndpointSlice) (*discoveryv1.EndpointSlice, error) {
	panic("to implement")
}
