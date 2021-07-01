package namespacesmapping

import (
	"strings"
)

type namespaceNotAvailable struct {
	namespaceName string
}

func (nnf *namespaceNotAvailable) Error() string {
	return strings.Join([]string{"namespace ", nnf.namespaceName, " cannot be retrieved in namespaceMap"}, "")
}
