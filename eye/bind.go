package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/grpc"
)

func bindEye(
	ctx context.Context,
	iris *Iris,
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
		Seal:  iris.Seal,
		Brand: iris.Brand,
	})
	if err != nil {
		return fmt.Errorf("bind Eye to Panopticon: %w", err)
	}

	if !response.GetSuccess() {
		return fmt.Errorf("Panopticon rejected Bind: %s", response.GetStatusMessage())
	}

	

	grantedBrand := response.GetBrand()

	if grantedBrand != "" {
		if err := imprintBrand(iris.StateDir, grantedBrand); err != nil {
			return fmt.Errorf("imprint granted Brand: %w", err)
		}

		iris.Brand = grantedBrand
		log.Printf("[EYE] Eye has been granted its Brand")
	}

	if iris.Seal != "" {
		iris.Seal = ""
		log.Printf("[EYE] Seal has been used and discarded")
	}

	return nil
}
