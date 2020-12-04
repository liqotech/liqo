package reflectors

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
)

type blackListType map[string]struct{}

// the blacklist is a map containing the objects that should not be managed by the reflectors.
// the blacklist is generally checked in the `isAllowed` method of the reflectors
// TODO: in a future version we could/should move to a dynamic blaklisting package with contexts
var Blacklist = map[apimgmt.ApiType]blackListType{
	apimgmt.EndpointSlices: {
		"default/kubernetes": struct{}{},
	},
	apimgmt.Pods: {},
	apimgmt.Services: {
		"default/kubernetes": struct{}{},
	},
}
