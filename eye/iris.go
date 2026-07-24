package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type Iris struct {
	PanopticonEndpoint   string
	PanopticonServerName string
	StateDir             string
}

func openIris() (Iris, error) {
	endpoint := strings.TrimSpace(os.Getenv("PANOPTICON_ENDPOINT"))

	if endpoint == "" {
		return Iris{}, fmt.Errorf("PANOPTICON_ENDPOINT is required")
	}

	serverName, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return Iris{}, fmt.Errorf(
			"PANOPTICON_ENDPOINT must be host:port: %w",
			err,
		)
	}

	return Iris{
		PanopticonEndpoint:   endpoint,
		PanopticonServerName: serverName,
		StateDir:             "./state",
	}, nil
}
