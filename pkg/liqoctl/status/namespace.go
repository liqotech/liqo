// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package status

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

const nsCheckerName = "namespace-existence"

// namespaceChecker implements the Check interface.
// checks if the namespace passed as an argument to liqoctl status command
// exists. If it does not exist the liqoctl status returns.
type namespaceChecker struct {
	client        k8s.Interface
	namespace     string
	name          string
	succeeded     bool
	failureReason error
}

func newNamespaceChecker(namespace string, client k8s.Interface) *namespaceChecker {
	return &namespaceChecker{
		client:    client,
		namespace: namespace,
		name:      nsCheckerName,
	}
}

func (nc *namespaceChecker) Collect(ctx context.Context) error {
	// Check if the namespace exists.
	if _, err := nc.client.CoreV1().Namespaces().Get(ctx, nc.namespace, v1.GetOptions{}); err != nil {
		nc.succeeded = false
		nc.failureReason = err
		return nil
	}

	nc.succeeded = true

	return nil
}

func (nc *namespaceChecker) Format() (string, error) {
	w, buf := newTabWriter(nc.name)

	if nc.succeeded {
		fmt.Fprintf(w, "%s liqo control plane namespace %s[%s]%s exists\n", checkMark, green, nc.namespace, reset)
	} else {
		fmt.Fprintf(w, "%s liqo control plane namespace %s[%s]%s is not OK\n", redCross, red, nc.namespace, reset)
		fmt.Fprintf(w, "Reason: %s\n", nc.failureReason)
	}

	// Add a new line ad the end of the message.
	fmt.Fprintf(w, "\n")
	if err := w.Flush(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (nc *namespaceChecker) HasSucceeded() bool {
	return nc.succeeded
}
