package main

import (
	"context"
	"fmt"

	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func bindEye(
	ctx context.Context,
	panopticonEndpoint string,
	eyeID string,
) error {
	connection, err := grpc.NewClient(
		panopticonEndpoint,
		// TODO: pwease change this to TLS comms later >_<
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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
