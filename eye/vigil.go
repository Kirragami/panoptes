package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/grpc"
)

func keepVigil(
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
		return fmt.Errorf("open Vigil connection to Panopticon: %w", err)
	}
	defer connection.Close()

	panopticon := proto.NewPanoptesServiceClient(connection)

	vigil, err := panopticon.KeepVigil(ctx)
	if err != nil {
		return fmt.Errorf("open Vigil stream: %w", err)
	}

	pulseInterval := 15 * time.Second
	pulseTicker := time.NewTicker(pulseInterval)
	defer pulseTicker.Stop()

	for {
		pulse := &proto.EyePulse{
			EyeId:      eyeID,
			SentAtUnix: time.Now().Unix(),
		}

		if err := vigil.Send(pulse); err != nil {
			return fmt.Errorf("send Eye pulse: %w", err)
		}

		signal, err := vigil.Recv()
		if err != nil {
			return fmt.Errorf("receive Panopticon signal: %w", err)
		}

		log.Printf("[EYE] %s", signal.GetMessage())

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-pulseTicker.C:
		}
	}
}

func maintainVigil(iris Iris, eyeID string) {
	retryDelay := time.Second
	maxRetryDelay := 30 * time.Second

	for {
		bindContext, cancelBind := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		err := bindEye(bindContext, &iris, eyeID)
		cancelBind()

		if err == nil {
			log.Printf("Eye successfully bound to Panopticon: %s", eyeID)

			retryDelay = time.Second

			err = keepVigil(
				context.Background(),
				&iris,
				eyeID,
			)
		}

		log.Printf(
			"Eye lost contact with Panopticon: %v; retrying in %s",
			err,
			retryDelay,
		)

		time.Sleep(retryDelay)

		retryDelay *= 2
		if retryDelay > maxRetryDelay {
			retryDelay = maxRetryDelay
		}
	}
}
