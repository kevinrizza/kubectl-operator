package portforward

import (
	"crypto/x509"
	"net/http"
	"time"

	"k8s.io/client-go/rest"
)

func NewHttpClient(restcfg *rest.Config, rootCAs *x509.CertPool) (*http.Client, error) {
	var err error
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig, err = rest.TLSConfigFor(restcfg)
	if err != nil {
		return nil, err
	}
	if rootCAs != nil {
		transport.TLSClientConfig.RootCAs = rootCAs
	}

	httpClient := http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return &httpClient, nil
}
