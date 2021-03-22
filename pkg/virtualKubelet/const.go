package virtualKubelet

type ContextKey string

const (
	VirtualNodePrefix       = "liqo-"
	VirtualKubeletPrefix    = "virtual-kubelet-"
	VirtualKubeletSecPrefix = "vk-kubeconfig-secret-"
	AdvertisementPrefix     = "advertisement-"
	ReflectedpodKey         = "virtualkubelet.liqo.io/source-pod"
	HomePodFinalizer        = "virtual-kubelet.liqo.io/provider"

	// Clients configuration
	HOME_CLIENT_QPS      = 1000
	HOME_CLIENTS_BURST   = 5000
	FOREIGN_CLIENT_QPS   = 1000
	FOREIGN_CLIENT_BURST = 5000
)
