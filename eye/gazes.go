package main

import (
	"context"
	"log"
	"strings"

	"github.com/Kirragami/panoptes/eye/visions"
	"github.com/Kirragami/panoptes/proto"
)

func heedGazes(
	ctx context.Context,
	registry *visions.Registry,
	heededTurns map[string]uint64,
	gazes []*proto.Gaze,
) {
	for _, gaze := range gazes {
		if gaze == nil {
			log.Printf("[EYE] Panopticon sent an empty Gaze")
			continue
		}

		sigil := strings.TrimSpace(gaze.GetSigil())
		if sigil == "" {
			log.Printf("[EYE] Panopticon sent a Gaze without a sigil")
			continue
		}

		if heededTurn, alreadyHeeded := heededTurns[sigil]; alreadyHeeded {
			if gaze.GetTurn() <= heededTurn {
				continue
			}
		}

		if err := registry.DiscernGaze(ctx, gaze); err != nil {
			log.Printf(
				"[EYE] Gaze %s remains unclear: %v",
				sigil,
				err,
			)
			continue
		}

		heededTurns[sigil] = gaze.GetTurn()

		log.Printf(
			"[EYE] Eye accepts Gaze %s at turn %d",
			sigil,
			gaze.GetTurn(),
		)
	}
}