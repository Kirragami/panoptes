package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Kirragami/panoptes/proto"
	"github.com/Kirragami/panoptes/eye/visions"
)

func keepVigil(
	ctx context.Context,
	iris *Iris,
	eyeID string,
	registry *visions.Registry,
) error {
	connection, err := openPanopticon(iris)
	if err != nil {
		return err
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

	heededTurns := make(map[string]uint64)

	for {
		pulse := &proto.EyePulse{
			EyeId:      eyeID,
			SentAtUnix: time.Now().Unix(),
			Brand:      iris.Brand,
		}

		if err := vigil.Send(pulse); err != nil {
			return fmt.Errorf("send Eye pulse: %w", err)
		}

		signal, err := vigil.Recv()
		if err != nil {
			return fmt.Errorf("receive Panopticon signal: %w", err)
		}

		heedGazes(
			ctx,
			registry,
			heededTurns,
			signal.GetGazes(),
		)

		log.Printf("[EYE] %s", signal.GetMessage())

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-pulseTicker.C:
		}
	}
}

func maintainVigil(
		iris Iris, 
		eyeID string,
		registry *visions.Registry,
	) {
	retryDelay := time.Second
	maxRetryDelay := 30 * time.Second

	for {
		bindContext, cancelBind := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		err := bindEye(bindContext, &iris, eyeID, registry)
		cancelBind()

		if err == nil {
			log.Printf("Eye successfully bound to Panopticon: %s", eyeID)

			retryDelay = time.Second

			err = keepVigil(
				context.Background(),
				&iris,
				eyeID,
				registry,
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
