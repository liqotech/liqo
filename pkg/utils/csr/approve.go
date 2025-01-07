// Copyright 2019-2025 The Liqo Authors
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

package csr

import (
	"context"

	certv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// Approve approves the provided CertificateSigningRequest.
func Approve(clientSet k8s.Interface, csr *certv1.CertificateSigningRequest, reason, message string) error {
	// certificate already added to CSR
	if csr.Status.Certificate != nil {
		return nil
	}
	// Check if the certificate is already approved but the certificate is still not available
	for _, b := range csr.Status.Conditions {
		if b.Type == "Approved" {
			return nil
		}
	}
	// Approve
	csr.Status.Conditions = append(csr.Status.Conditions, certv1.CertificateSigningRequestCondition{
		Type:           certv1.CertificateApproved,
		Reason:         reason,
		Message:        message,
		LastUpdateTime: metav1.Now(),
		Status:         corev1.ConditionTrue,
	})
	_, errApproval := clientSet.CertificatesV1().CertificateSigningRequests().
		UpdateApproval(context.TODO(), csr.Name, csr, metav1.UpdateOptions{})
	if errApproval != nil {
		return errApproval
	}
	return nil
}

// ApproverHandler returns an handler to approve CSRs.
func ApproverHandler(clientset k8s.Interface, reason, message string,
	filter func(csr *certv1.CertificateSigningRequest) bool) func(*certv1.CertificateSigningRequest) {
	return func(csr *certv1.CertificateSigningRequest) {
		if filter(csr) {
			if err := Approve(clientset, csr, reason, message); err != nil {
				klog.Error(err)
			} else {
				klog.Infof("CSR %v correctly approved", csr.Name)
			}
		}
	}
}
