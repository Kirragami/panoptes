package main

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
)

func openPanopticon(
	iris *Iris,
) (*grpc.ClientConn, error) {
	connection, err := grpc.NewClient(
		"passthrough:///"+iris.PanopticonEndpoint,
		grpc.WithTransportCredentials(openAegis(iris)),
		grpc.WithContextDialer(
			func(ctx context.Context, address string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(
					ctx,
					"tcp4",
					address,
				)
			},
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open connection to Panopticon: %w",
			err,
		)
	}

	return connection, nil
}