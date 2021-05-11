package wireguard

import (
	"context"
	"fmt"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
)

// GetKeys it checks if a the secret with the given name exists and retrieves the wireguard keys.
// If the secret does not exist than it is creates and newly generated keys are stored on it.
func GetKeys(secretName, namespace string, c k8s.Interface) (priv, pub wgtypes.Key, err error) {
	// first we check if a secret containing valid keys already exists
	s, err := c.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return priv, pub, err
	}
	// if the secret does not exist then keys are generated and saved into a secret
	if apierrors.IsNotFound(err) {
		// generate private and public keys
		if priv, err = wgtypes.GeneratePrivateKey(); err != nil {
			return priv, pub, fmt.Errorf("error generating private key for wireguard backend: %w", err)
		}
		pub = priv.PublicKey()
		pKey := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			StringData: map[string]string{wireguard.PublicKey: pub.String(), wireguard.PrivateKey: priv.String()},
		}
		_, err = c.CoreV1().Secrets(namespace).Create(context.Background(), &pKey, metav1.CreateOptions{})
		if err != nil {
			return priv, pub, fmt.Errorf("failed to create the secret with name %s: %w", secretName, err)
		}
		return priv, pub, nil
	}
	// get the keys from the existing secret and set them
	privKey, found := s.Data[wireguard.PrivateKey]
	if !found {
		return priv, pub, fmt.Errorf("no data with key '%s' found in secret %s", wireguard.PrivateKey, secretName)
	}
	priv, err = wgtypes.ParseKey(string(privKey))
	if err != nil {
		return priv, pub, fmt.Errorf("an error occurred while parsing the private key for the wireguard driver :%w", err)
	}
	pubKey, found := s.Data[wireguard.PublicKey]
	if !found {
		return priv, pub, fmt.Errorf("no data with key '%s' found in secret %s", wireguard.PublicKey, secretName)
	}
	pub, err = wgtypes.ParseKey(string(pubKey))
	if err != nil {
		return priv, pub, fmt.Errorf("an error occurred while parsing the public key for the wireguard driver :%w", err)
	}
	return priv, pub, nil
}
