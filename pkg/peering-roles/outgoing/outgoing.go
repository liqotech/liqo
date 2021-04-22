package outgoing

// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=get;update;patch;list;watch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs/status,verbs=get;update;patch;list;watch;delete

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourceoffers,verbs=get;update;patch;list;watch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourceoffers/status,verbs=get;update;patch;list;watch;delete
