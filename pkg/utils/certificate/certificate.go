// Copyright 2019-2026 The Liqo Authors
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

// Package certificate provides utilities for certificate handling and renewal.
package certificate

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"k8s.io/klog/v2"
)

// ShouldRenewCertificate calculates the certificate's lifetime and determines if a renewal is required
// based on the 2/3 life rule. If the certificate does not need to be renewed, it also returns the duration
// until the next renewal check should be performed.
func ShouldRenewCertificate(pemCert []byte) (bool, time.Duration, error) {
	block, _ := pem.Decode(pemCert)
	if block == nil {
		return false, 0, fmt.Errorf("failed to decode PEM block containing certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, 0, fmt.Errorf("failed to parse certificate: %w", err)
	}

	requeueIn := NextRenewalCheck(cert.NotBefore, cert.NotAfter)
	if requeueIn <= 0 {
		return true, 0, nil
	}

	klog.V(4).Infof("Certificate not ready for renewal, will check again in %v", requeueIn)
	return false, requeueIn, nil
}

// NextRenewalCheck returns the duration until the next certificate renewal check
// based on the 2/3 lifetime rule with a 10% buffer.
func NextRenewalCheck(notBefore, notAfter time.Time) time.Duration {
	lifetime := notAfter.Sub(notBefore)
	twoThirdsPoint := notAfter.Add(-lifetime / 3)
	return time.Until(twoThirdsPoint) * 11 / 10
}
