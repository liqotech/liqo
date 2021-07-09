package foreignclusteroperator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery/utils"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// check if the ForeignCluster CR does not have a value in one of the required fields (Namespace and ClusterID)
// and needs a value defaulting.
func (r *ForeignClusterReconciler) needsClusterIdentityDefaulting(fc *v1alpha1.ForeignCluster) bool {
	return fc.Spec.ClusterIdentity.ClusterID == ""
}

// load the default values for that ForeignCluster basing on the AuthUrl value, an HTTP request is sent and the retrieved
// values are applied for the following fields (if they are empty): Namespace, ClusterIdentity.ClusterID, ClusterIdentity.Namespace
// and the TrustMode
// if it returns no error, the ForeignCluster CR has been updated.
func (r *ForeignClusterReconciler) clusterIdentityDefaulting(fc *v1alpha1.ForeignCluster) error {
	klog.V(4).Infof("Defaulting ClusterIdentity values for ForeignCluster %v", fc.Name)
	ids, err := utils.GetClusterInfo(foreignclusterutils.InsecureSkipTLSVerify(fc), fc.Spec.ForeignAuthURL)
	if err != nil {
		klog.Error(err)
		return err
	}

	if fc.Spec.ClusterIdentity.ClusterID == "" {
		fc.Spec.ClusterIdentity.ClusterID = ids.ClusterID
	}
	if fc.Spec.ClusterIdentity.ClusterName == "" {
		fc.Spec.ClusterIdentity.ClusterName = ids.ClusterName
	}

	klog.V(4).Infof("New values:\n\tClusterId:\t%v\n\tClusterName:\t%v",
		fc.Spec.ClusterIdentity.ClusterID,
		fc.Spec.ClusterIdentity.ClusterName)

	// update the ForeignCluster
	if _, err = r.crdClient.Resource("foreignclusters").Update(fc.Name, fc, &metav1.UpdateOptions{}); err != nil {
		klog.Error(err)
		return err
	}
	return nil
}
