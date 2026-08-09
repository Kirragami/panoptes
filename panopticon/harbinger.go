package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

type Harbinger interface {
	BearOmen(
		context.Context,
		OmenRecord,
	) error
}

type LogHarbinger struct{}

func awakenHarbinger(
	ctx context.Context,
) (Harbinger, error) {
	credentialsPath := strings.TrimSpace(
		os.Getenv("PANOPTICON_FIREBASE_CREDENTIALS"),
	)

	deviceToken := strings.TrimSpace(
		os.Getenv("PANOPTICON_FCM_DEVICE_TOKEN"),
	)

	if credentialsPath == "" && deviceToken == "" {
		return LogHarbinger{}, nil
	}

	if credentialsPath == "" || deviceToken == "" {
		return nil, fmt.Errorf(
			"PANOPTICON_FIREBASE_CREDENTIALS and PANOPTICON_FCM_DEVICE_TOKEN are required",
		)
	}

	return NewFCMHarbinger(
		ctx,
		credentialsPath,
		deviceToken,
	)
}

func (LogHarbinger) BearOmen(
	_ context.Context,
	omen OmenRecord,
) error {
	// add notification to mobile app here
	log.Printf(
		"[PANOPTICON] Harbinger: An Omen has befallen upon %s from Eye %s",
		omen.GazeSigil,
		omen.EyeID,
	)

	return nil
}
