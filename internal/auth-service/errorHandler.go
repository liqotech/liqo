package authservice

import (
	"net/http"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	autherrors "github.com/liqotech/liqo/pkg/auth/errors"
)

func (authService *Controller) handleError(w http.ResponseWriter, err error) {
	switch err.(type) {
	case *kerrors.StatusError:
		authService.sendError(w, err.Error(), http.StatusForbidden)
	case *autherrors.ClientError:
		authService.sendError(w, err.Error(), http.StatusBadRequest)
	case *autherrors.AuthenticationFailedError:
		authService.sendError(w, err.Error(), http.StatusUnauthorized)
	default:
		authService.sendError(w, err.Error(), http.StatusInternalServerError)
	}
}

func (authService *Controller) sendError(w http.ResponseWriter, resp string, code int) {
	klog.V(3).Infof("%v - sending error response: %v", code, resp)
	http.Error(w, resp, code)
}
