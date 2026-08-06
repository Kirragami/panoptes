package main

import (
	"crypto/tls"

	"google.golang.org/grpc/credentials"
)

func openAegis(iris *Iris) credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		ServerName: iris.PanopticonServerName,
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{"h2"},
	})
}
