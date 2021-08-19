package add

import (
	"context"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/liqoctl/common"
	utils2 "github.com/liqotech/liqo/pkg/utils"
	authenticationtokenutils "github.com/liqotech/liqo/pkg/utils/authenticationtoken"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// ClusterArgs encapsulates arguments required to enable an outgoing peering to a remote cluster.
type ClusterArgs struct {
	ClusterName    string
	ClusterToken   string
	ClusterAuthURL string
	ClusterID      string
	Namespace      string
}

// HandleAddCommand handles the add command, configuring all the resources required to configure an outgoing peering.
func HandleAddCommand(ctx context.Context, t *ClusterArgs) {
	restConfig := common.GetLiqoctlRestConfOrDie()

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Fatalf(err.Error())
	}

	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		klog.Fatalf(err.Error())
	}

	if err := processAddCluster(ctx, t, clientSet, k8sClient); err != nil {
		klog.Fatalf(err.Error())
	}
}

func processAddCluster(ctx context.Context, t *ClusterArgs, clientSet kubernetes.Interface, k8sClient client.Client) error {
	// Create Secret
	err := authenticationtokenutils.StoreInSecret(ctx, clientSet, t.ClusterID, t.ClusterToken, t.Namespace)
	if err != nil {
		return err
	}

	clusterID, err := utils2.GetClusterIDWithControllerClient(ctx, k8sClient, t.Namespace)
	if err != nil {
		return err
	}
	// Check clusterIDs are not equal. If they are, abort.
	if clusterID == t.ClusterID {
		return fmt.Errorf(sameClusterError)
	}

	err = enforceForeignCluster(ctx, k8sClient, t)
	if err != nil {
		return err
	}
	return nil
}

func enforceForeignCluster(ctx context.Context, cl client.Client, t *ClusterArgs) error {
	// Create ForeignCluster
	fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, t.ClusterID)
	if kerrors.IsNotFound(err) {
		fc = &discoveryv1alpha1.ForeignCluster{ObjectMeta: metav1.ObjectMeta{Name: t.ClusterName,
			Labels: map[string]string{discovery.ClusterIDLabel: t.ClusterID}}}
	} else if err != nil {
		return err
	}

	_, err = controllerutil.CreateOrUpdate(ctx, cl, fc, func() error {
		fc.Spec.ForeignAuthURL = t.ClusterAuthURL
		fc.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledYes
		if fc.Spec.IncomingPeeringEnabled == "" {
			fc.Spec.IncomingPeeringEnabled = discoveryv1alpha1.PeeringEnabledAuto
		}
		if fc.Spec.InsecureSkipTLSVerify == nil {
			fc.Spec.InsecureSkipTLSVerify = pointer.BoolPtr(true)
		}
		return nil
	})
	return err
}
