package foreigncluster

import discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"

// InsecureSkipTLSVerify returns true if the ForeignCluster has to be contacted without the TLS verification.
func InsecureSkipTLSVerify(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return foreignCluster.Spec.InsecureSkipTLSVerify != nil && *foreignCluster.Spec.InsecureSkipTLSVerify
}
