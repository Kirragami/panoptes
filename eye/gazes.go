package main

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/Kirragami/panoptes/eye/visions"
	"github.com/Kirragami/panoptes/proto"
)

type GazeKeeper struct {
	iris     *Iris
	eyeID    string
	registry *visions.Registry

	mu     sync.Mutex
	turns  map[string]uint64
	vigils map[string]visions.Vigil
}

func newGazeKeeper(
	iris *Iris,
	eyeID string,
	registry *visions.Registry,
) *GazeKeeper {
	return &GazeKeeper{
		iris:     iris,
		eyeID:    eyeID,
		registry: registry,
		turns:    make(map[string]uint64),
		vigils:   make(map[string]visions.Vigil),
	}
}

func (k *GazeKeeper) heed(
	ctx context.Context,
	gazes []*proto.Gaze,
) {
	for _, gaze := range gazes {
		k.heedOne(ctx, gaze)
	}
}

func (k *GazeKeeper) heedOne(
	ctx context.Context,
	gaze *proto.Gaze,
) {
	if gaze == nil {
		log.Printf("[EYE] Panopticon sent an empty Gaze")
		return
	}

	sigil := strings.TrimSpace(gaze.GetSigil())
	if sigil == "" {
		log.Printf("[EYE] Panopticon sent a Gaze without a sigil")
		return
	}

	k.mu.Lock()
	knownTurn, known := k.turns[sigil]
	k.mu.Unlock()

	if known && gaze.GetTurn() <= knownTurn {
		return
	}

	if err := k.registry.DiscernGaze(ctx, gaze); err != nil {
		log.Printf(
			"[EYE] Gaze %s remains unclear: %v",
			sigil,
			err,
		)
		return
	}

	vision, _ := k.registry.Recall(gaze.GetVision())

	var nextVigil visions.Vigil

	if gaze.GetAwake() {
		gazingVision, canGaze := vision.(visions.GazingVision)
		if !canGaze {
			log.Printf(
				"[EYE] Vision %s cannot awaken a Gaze",
				gaze.GetVision(),
			)
			return
		}

		vigil, err := gazingVision.Awaken(
			ctx,
			gaze,
			k.raise,
		)
		if err != nil {
			log.Printf(
				"[EYE] Failed to awaken Gaze %s: %v",
				sigil,
				err,
			)
			return
		}

		nextVigil = vigil
	}

	k.mu.Lock()

	currentTurn, current := k.turns[sigil]
	if current && currentTurn >= gaze.GetTurn() {
		k.mu.Unlock()

		if nextVigil != nil {
			nextVigil.Sleep()
		}

		return
	}

	oldVigil := k.vigils[sigil]

	k.turns[sigil] = gaze.GetTurn()

	if nextVigil == nil {
		delete(k.vigils, sigil)
	} else {
		k.vigils[sigil] = nextVigil
	}

	k.mu.Unlock()

	if oldVigil != nil {
		oldVigil.Sleep()
	}

	if gaze.GetAwake() {
		log.Printf(
			"[EYE] Eye awakens Gaze %s at turn %d",
			sigil,
			gaze.GetTurn(),
		)
		return
	}

	log.Printf(
		"[EYE] Eye lets Gaze %s sleep at turn %d",
		sigil,
		gaze.GetTurn(),
	)
}

func (k *GazeKeeper) raise(
	ctx context.Context,
	gaze *proto.Gaze,
) error {
	omen, err := foretellOmen(
		k.iris,
		k.eyeID,
		gaze,
	)
	if err != nil {
		return err
	}

	return raiseOmen(ctx, k.iris, omen)
}

func (k *GazeKeeper) SleepAll() {
	k.mu.Lock()

	active := make([]visions.Vigil, 0, len(k.vigils))
	for _, vigil := range k.vigils {
		active = append(active, vigil)
	}

	k.vigils = make(map[string]visions.Vigil)
	k.turns = make(map[string]uint64)

	k.mu.Unlock()

	for _, vigil := range active {
		vigil.Sleep()
	}
}