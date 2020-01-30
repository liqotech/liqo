/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"os"
	//"runtime"
	//"time"

	"github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"

	"github.com/pkg/errors"
	//"k8s.io/apimachinery/pkg/api/resource"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	advertisementv1beta1 "github.com/netgroup-polito/dronev2/advertisement-operator/api/v1beta1"
	"github.com/netgroup-polito/dronev2/advertisement-operator/controllers"

	// +kubebuilder:scaffold:imports
)

var (
	scheme   = k8sruntime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = advertisementv1beta1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.AdvertiserReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Advertiser"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Advertiser")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	//go generateAdvertisement()
}

/*func generateAdvertisement() error{

	freeResources := advertisementv1beta1.FreeResource{
		Cpu: *resource.NewQuantity(int64(runtime.NumCPU()), resource.DecimalSI),
		CpuPrice: *resource.NewQuantity(0.0012, resource.DecimalSI),
		Ram: *resource.NewQuantity(2000, resource.DecimalSI),
		RamPrice: *resource.NewQuantity(0.23, resource.DecimalSI),
	}

	adv := advertisementv1beta1.Advertiser{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       advertisementv1beta1.AdvertiserSpec{
			ClusterId:	  "cluster1",
			Resources: 	  getDockerImages(),
			Availability: freeResources,
			Timestamp:    metav1.NewTime(time.Now()),
			Validity:     metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}

	println(adv)

	return nil
	//apiextension.Clientset{}
	//remoteClient, err := newClient("./data/foreignKubeconfig")
	//	if err != nil {
	//		return err
	//	}

}
*/
func getDockerImages() []advertisementv1beta1.Resource {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
	if err != nil {
		panic(err)
	}

	dockerImages, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		panic(err)
	}

	images := make([]advertisementv1beta1.Resource, len(dockerImages))

	/*for i := 0 ; i < len(dockerImages) ; i++ {
		images[i].Image.Names = append(images[i].Image.Names, dockerImages[i].ID)
		images[i].Image.SizeBytes = dockerImages[i].Size
		images[i].Price = *resource.NewQuantity(0.5, resource.DecimalSI)
	}
*/
	return images
}

func newClient(configPath string) (*kubernetes.Clientset, error) {
	var config *rest.Config

	// Check if the kubeConfig file exists.
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil, errors.Wrap(err, "error building client config")
		}
	} else {
		// Set to in-cluster config.
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, errors.Wrap(err, "error building in cluster config")
		}
	}

	if masterURI := os.Getenv("MASTER_URI"); masterURI != "" {
		config.Host = masterURI
	}


	return kubernetes.NewForConfig(config)
}
