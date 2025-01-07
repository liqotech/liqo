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

package forge

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/pointer"
)

const (
	// LiqoSASecretForPodNameKey is the key of a label identifying the name of the pod associated with the given service account tokens.
	LiqoSASecretForPodNameKey = "offloading.liqo.io/service-account-for-pod-name"
	// LiqoSASecretForServiceAccountKey is the key of a label identifying the name of the service account originating the given tokens.
	LiqoSASecretForServiceAccountKey = "offloading.liqo.io/service-account-name"
	// LiqoSASecretForPodUIDKey is the key of an annotation identifying the uid of the pod associated with the given service account tokens.
	LiqoSASecretForPodUIDKey = "offloading.liqo.io/service-account-for-pod-uid"
	// LiqoSASecretExpirationKey is the key of an annotation storing the expiration timestamp of the given service account tokens.
	LiqoSASecretExpirationKey = "offloading.liqo.io/service-account-expiration"

	// TokenRefreshAtLifespanPercentage is the percentage of the token lifespan when it should be refreshed.
	TokenRefreshAtLifespanPercentage = 80.0
)

var serviceAccountSecretSelector labels.Selector

// Initialize the selector at startup time, for the sake of efficiency.
func init() {
	req1, err := labels.NewRequirement(LiqoSASecretForPodNameKey, selection.Exists, nil)
	utilruntime.Must(err)
	req2, err := labels.NewRequirement(LiqoSASecretForServiceAccountKey, selection.Exists, nil)
	utilruntime.Must(err)
	serviceAccountSecretSelector = labels.NewSelector().Add(*req1, *req2)
}

// IsServiceAccountSecret returns whether the current object contains remotely reflected service account tokens.
func IsServiceAccountSecret(obj metav1.Object) bool {
	return serviceAccountSecretSelector.Matches(labels.Set(obj.GetLabels()))
}

// RemoteSecret forges the apply patch for the reflected secret, given the local one.
func RemoteSecret(local *corev1.Secret, targetNamespace string, forgingOpts *ForgingOpts) *corev1apply.SecretApplyConfiguration {
	applyConfig := corev1apply.Secret(local.GetName(), targetNamespace).
		WithLabels(FilterNotReflected(local.GetLabels(), forgingOpts.LabelsNotReflected)).WithLabels(ReflectionLabels()).
		WithAnnotations(FilterNotReflected(local.GetAnnotations(), forgingOpts.AnnotationsNotReflected)).
		WithData(local.Data).
		WithType(local.Type)

	if local.Immutable != nil {
		applyConfig = applyConfig.WithImmutable(*local.Immutable)
	}

	// It is not possible to create a ServiceAccountToken secret if the corresponding
	// service account does not exist, hence it is mutated to an opaque secret.
	// In addition, we also add a label with the service account name, for easier retrieval.
	if local.Type == corev1.SecretTypeServiceAccountToken {
		applyConfig = applyConfig.WithType(corev1.SecretTypeOpaque).
			WithLabels(map[string]string{corev1.ServiceAccountNameKey: local.Annotations[corev1.ServiceAccountNameKey]})
	}

	return applyConfig
}

// RemoteServiceAccountSecret forges the apply patch for the secret containing the service account token, given the token request.
func RemoteServiceAccountSecret(tokens *ServiceAccountPodTokens, targetName, targetNamespace, nodename string) *corev1apply.SecretApplyConfiguration {
	return corev1apply.Secret(targetName, targetNamespace).
		WithLabels(ReflectionLabels()).WithLabels(RemoteServiceAccountSecretLabels(tokens)).
		WithLabels(map[string]string{LiqoOriginClusterNodeName: nodename}).
		WithAnnotations(RemoteServiceAccountSecretAnnotations(tokens)).
		WithStringData(tokens.TokensForSecret()).
		WithType(corev1.SecretTypeOpaque).
		WithImmutable(false)
}

// RemoteServiceAccountSecretLabels returns the labels assigned to the secret holding service account tokens.
func RemoteServiceAccountSecretLabels(tokens *ServiceAccountPodTokens) labels.Set {
	return map[string]string{
		LiqoSASecretForPodNameKey:        tokens.PodName,
		LiqoSASecretForServiceAccountKey: tokens.ServiceAccountName,
	}
}

// RemoteServiceAccountSecretAnnotations returns the annotations assigned to the secret holding service account tokens.
func RemoteServiceAccountSecretAnnotations(tokens *ServiceAccountPodTokens) labels.Set {
	return map[string]string{
		LiqoSASecretForPodUIDKey:  string(tokens.PodUID),
		LiqoSASecretExpirationKey: tokens.EarliestExpiration().Format(time.RFC3339),
	}
}

// ServiceAccountSecretName returns the name of the ServiceAccount secret associated to a given pod and volume.
func ServiceAccountSecretName(podName string) string {
	return podName + "-token"
}

// ServiceAccountTokenKey returns the key to identify a given token.
func ServiceAccountTokenKey(volumeName, path string) string {
	hash := sha256.Sum256([]byte(path))
	// We want 6 chars, so we encode 3 bytes
	suffix := hex.EncodeToString(hash[:3])
	return volumeName + "-" + suffix
}

// ServiceAccountTokenFromSecret retrieves the token corresponding to the given key from a secret.
func ServiceAccountTokenFromSecret(secret *corev1.Secret, key string) string {
	if secret != nil {
		if value, ok := secret.Data[key]; ok {
			return string(value)
		}
	}

	return ""
}

// ServiceAccountPodUIDFromSecret retrieves the UID of the corresponding pod from a secret, or podUID if nil.
func ServiceAccountPodUIDFromSecret(secret *corev1.Secret, podUID types.UID) types.UID {
	if secret == nil {
		return podUID
	}

	return types.UID(secret.GetAnnotations()[LiqoSASecretForPodUIDKey])
}

// ServiceAccountTokenExpirationFromSecret retrieves the earliest token expiration from a secret.
func ServiceAccountTokenExpirationFromSecret(secret *corev1.Secret) time.Time {
	if secret != nil {
		if value, ok := secret.Annotations[LiqoSASecretExpirationKey]; ok {
			if parsed, err := time.Parse(time.RFC3339, value); err == nil {
				return parsed
			}
		}
	}

	return time.Time{}
}

// ServiceAccountPodTokens constains the information for the service account tokens associated with a pod.
type ServiceAccountPodTokens struct {
	PodName            string
	PodUID             types.UID
	ServiceAccountName string

	Tokens []*ServiceAccountPodToken
}

// ServiceAccountPodToken contains the information corresponding to a service account token associated with a pod.
type ServiceAccountPodToken struct {
	Key string

	Audiences         []string
	ExpirationSeconds int64

	Token            string
	ActualExpiration time.Time
}

// AddToken appends the information corresponding to a given service account token.
func (tokens *ServiceAccountPodTokens) AddToken(key, audience string, expiration int64) *ServiceAccountPodToken {
	var audiences []string
	if audience != "" {
		audiences = []string{audience}
	}

	token := &ServiceAccountPodToken{
		Key:               key,
		Audiences:         audiences,
		ExpirationSeconds: expiration,
	}

	tokens.Tokens = append(tokens.Tokens, token)
	return token
}

// TokensForSecret returns a map with keys the volume name, and value the corresponding service account token.
func (tokens *ServiceAccountPodTokens) TokensForSecret() map[string]string {
	mappings := make(map[string]string)

	for _, token := range tokens.Tokens {
		mappings[token.Key] = token.Token
	}

	return mappings
}

// EarliestExpiration returns the earliest expiration of all considered tokens.
func (tokens *ServiceAccountPodTokens) EarliestExpiration() time.Time {
	return tokens.earliestExpirationWithDelta(0.0)
}

// EarliestRefresh returns the timestamp at which the first token should be refreshed.
func (tokens *ServiceAccountPodTokens) EarliestRefresh() time.Time {
	return tokens.earliestExpirationWithDelta(100. - TokenRefreshAtLifespanPercentage)
}

// EarliestExpirationWithDelta returns the earliest expiration (decremented by the given percentage of its duration) of all considered tokens.
func (tokens *ServiceAccountPodTokens) earliestExpirationWithDelta(percentage float32) time.Time {
	var earliest time.Time

	for _, token := range tokens.Tokens {
		refresh := token.expirationWithDelta(percentage)
		if earliest.IsZero() || refresh.Before(earliest) {
			earliest = refresh
		}
	}

	return earliest
}

// TokenRequest returns a new TokenRequest based on the given TokenInfo.
func (token *ServiceAccountPodToken) TokenRequest(ref *corev1.Pod) *authenticationv1.TokenRequest {
	return &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         token.Audiences,
			ExpirationSeconds: pointer.Int64(token.ExpirationSeconds),
			BoundObjectRef: &authenticationv1.BoundObjectReference{
				APIVersion: corev1.SchemeGroupVersion.Version,
				Kind:       "Pod",
				Name:       ref.GetName(),
				UID:        ref.GetUID(),
			},
		},
	}
}

// Update updates the TokenInfo based on the TokenRequest response.
func (token *ServiceAccountPodToken) Update(tkn string, expiration time.Time) {
	token.Token = tkn
	token.ActualExpiration = expiration
}

// RefreshDue returns the timestamp at which the token should be refreshed.
func (token *ServiceAccountPodToken) RefreshDue() time.Time {
	// Tokens should be refreshed when more than 80% of its lifespan passed.
	return token.expirationWithDelta(100.0 - TokenRefreshAtLifespanPercentage)
}

// expirationWithDelta computes the expiration timestamp of the token, decremented by the given percentage of its duration.
func (token *ServiceAccountPodToken) expirationWithDelta(percentage float32) time.Time {
	return token.ActualExpiration.Add(time.Duration(-0.01*float32(token.ExpirationSeconds)*percentage) * time.Second)
}
