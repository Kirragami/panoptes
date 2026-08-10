package main

import (
	"context"
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
	chronicle *Chronicle,
) (Harbinger, error) {
	credentialsPath := strings.TrimSpace(
		os.Getenv("PANOPTICON_FIREBASE_CREDENTIALS"),
	)

	if credentialsPath == "" {
		return LogHarbinger{}, nil
	}

	return NewFCMHarbinger(
		ctx,
		credentialsPath,
		chronicle,
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
