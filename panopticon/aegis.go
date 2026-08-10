package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc/credentials"
)

func loadPanopticonCertificate() (tls.Certificate, error) {
	certificateFile := strings.TrimSpace(
		os.Getenv("PANOPTICON_TLS_CERT_FILE"),
	)

	keyFile := strings.TrimSpace(
		os.Getenv("PANOPTICON_TLS_KEY_FILE"),
	)

	if certificateFile == "" {
		return tls.Certificate{}, fmt.Errorf(
			"PANOPTICON_TLS_CERT_FILE is required",
		)
	}

	if keyFile == "" {
		return tls.Certificate{}, fmt.Errorf(
			"PANOPTICON_TLS_KEY_FILE is required",
		)
	}

	certificate, err := tls.LoadX509KeyPair(
		certificateFile,
		keyFile,
	)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf(
			"load Panopticon certificate and key: %w",
			err,
		)
	}

	return certificate, nil
}

func raiseAegis() (credentials.TransportCredentials, error) {
	certificate, err := loadPanopticonCertificate()
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h2"},
	}), nil
}
