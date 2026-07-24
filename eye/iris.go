package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type Iris struct {
	PanopticonEndpoint string
	StateDir           string
}

func openIris() (Iris, error) {
	endpoint := strings.TrimSpace(os.Getenv("PANOPTICON_ENDPOINT"))

	if endpoint == "" {
		return Iris{}, fmt.Errorf("PANOPTICON_ENDPOINT is required")
	}

	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		return Iris{}, fmt.Errorf(
			"PANOPTICON_ENDPOINT must be host:port: %w",
			err,
		)
	}

	return Iris{
		PanopticonEndpoint: endpoint,
		StateDir:           "./state",
	}, nil
}
