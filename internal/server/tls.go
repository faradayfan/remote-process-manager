package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

func BuildMTLSServerConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read ca_file %q: %w", caFile, err)
	}

	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("failed to parse CA certs from %q", caFile)
	}

	serverCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load server cert/key %q/%q: %w", certFile, keyFile, err)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{serverCert},

		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  caPool,
	}, nil
}
