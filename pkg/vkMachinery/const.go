package vkMachinery

import "path/filepath"

const VKCertsRootPath = "/etc/virtual-kubelet/certs"

var KeyLocation = filepath.Join(VKCertsRootPath, "server-key.pem")
var CertLocation = filepath.Join(VKCertsRootPath, "server.crt")
var CsrLocation = filepath.Join(VKCertsRootPath, "server.csr")
var CsrLabels = map[string]string{
	"virtual-kubelet.liqo.io/csr": "true",
}

var KubeletBaseLabels = map[string]string{
	"app.kubernetes.io/name":       "virtual-kubelet",
	"app.kubernetes.io/instance":   "virtual-kubelet",
	"app.kubernetes.io/managed-by": "advertisement-operator",
	"app.kubernetes.io/component":  "virtual-kubelet",
	"app.kubernetes.io/part-of":    "liqo",
}
