// Package kubernetes wraps a set of packages copied verbatim (possibly removing unused functions) from upstream
// kubernetes, so that we do not have to import from k8s.io/kubernetes, which is currently problematic.
// Specifically, they include:
// * envvars -> k8s.io/kubernetes/pkg/kubelet/envvars
package kubernetes
