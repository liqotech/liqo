package outgoing

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
)

type blackList map[string]bool

var blacklist = map[apimgmt.ApiType]blackList{
	apimgmt.EndpointSlices: {
		"default/kubernetes": true,
	},
	apimgmt.Services: {
		"default/kubernetes": true,
	},
}
