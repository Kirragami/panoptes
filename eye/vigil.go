package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func keepVigil(
	ctx context.Context,
	panopticonEndpoint string,
	eyeID string,
) error {
	connection, err := grpc.NewClient(
		panopticonEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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
