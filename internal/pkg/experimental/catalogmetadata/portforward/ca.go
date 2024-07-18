package portforward

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetClusterCA returns an x509.CertPool by reading the contents of a Kubernetes Secret. It uses the provided
// client to get the requested secret and then loads the contents of the secret's "ca.crt" key into the cert pool.
func GetClusterCA(ctx context.Context, cl client.Reader) (*x509.CertPool, error) {
	cmConnectionDetails := &corev1.ConfigMap{}
	configmapKey := types.NamespacedName{
		Namespace: "olmv1-system",
		Name:      "catalogd-connection-details",
	}
	if err := cl.Get(ctx, configmapKey, cmConnectionDetails); err != nil {
		return nil, fmt.Errorf("get connection details config data from catalogd: %v", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(cmConnectionDetails.BinaryData["ca"]) {
		return nil, errors.New("failed to load certificate authority into cert pool: malformed PEM?")
	}
	return certPool, nil
}
