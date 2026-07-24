package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc/credentials"
)

func raiseAegis() (credentials.TransportCredentials, error) {
	certificateFile := strings.TrimSpace(
		os.Getenv("PANOPTICON_TLS_CERT_FILE"),
	)

	keyFile := strings.TrimSpace(
		os.Getenv("PANOPTICON_TLS_KEY_FILE"),
	)

	if certificateFile == "" {
		return nil, fmt.Errorf("PANOPTICON_TLS_CERT_FILE is required")
	}

	if keyFile == "" {
		return nil, fmt.Errorf("PANOPTICON_TLS_KEY_FILE is required")
	}

	certificate, err := tls.LoadX509KeyPair(
		certificateFile,
		keyFile,
	)
	if err != nil {
		return nil, fmt.Errorf("load Panopticon certificate and key: %w", err)
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h2"},
	}), nil
}
