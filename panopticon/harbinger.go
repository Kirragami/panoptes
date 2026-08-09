package main

import (
	"context"
	"log"
)

type Harbinger interface {
	BearOmen(
		context.Context,
		OmenRecord,
	) error
}

type LogHarbinger struct{}

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
