package main

import (
	"context"
	"fmt"
	"net"

	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/grpc"
)

func bindEye(
	ctx context.Context,
	iris Iris,
	eyeID string,
) error {
	connection, err := grpc.NewClient(
		"passthrough:///"+iris.PanopticonEndpoint,
		grpc.WithTransportCredentials(openAegis(iris)),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp4", addr)
		}),
	)
	if err != nil {
		return fmt.Errorf("open connection to Panopticon: %w", err)
	}
	defer connection.Close()

	panopticon := proto.NewPanoptesServiceClient(connection)

	response, err := panopticon.BindEye(ctx, &proto.BindRequest{
		EyeId: eyeID,
	})
	if err != nil {
		return fmt.Errorf("bind Eye to Panopticon: %w", err)
	}

	if !response.GetSuccess() {
		return fmt.Errorf("Panopticon rejected Bind: %s", response.GetStatusMessage())
	}

	return nil
}
