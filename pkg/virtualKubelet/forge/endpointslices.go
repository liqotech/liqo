package forge

import (
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
)

func (f *apiForger) endpointsliceHomeToForeign(homeEndpointslice, foreignEndpointslice *discoveryv1beta1.EndpointSlice) (*discoveryv1beta1.EndpointSlice, error) {
	panic("to implement")
}
