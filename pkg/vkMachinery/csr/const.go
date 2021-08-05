package csr

const (
	kubeletServingSignerName    = "kubernetes.io/kubelet-serving"
	kubeletAPIServingSignerName = "kubernetes.io/kube-apiserver-client-kubelet"
)

const (
	csrSecretLabel = "liqo.io/virtual-kubelet-csr-secret" // nolint:gosec // not a credential
)
