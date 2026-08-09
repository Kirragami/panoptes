package visions

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// lets just go with this design from start for better scalability teehee
type Vision interface {
	Sight() string
	Form() uint32
	Behold(context.Context) (awake bool, slumberReason string, err error)
	DiscernFocus(*structpb.Struct) error
}

type Registry struct {
	visions map[string]Vision
	order   []string
}

func NewRegistry(entries ...Vision) (*Registry, error) {
	registry := &Registry{
		visions: make(map[string]Vision),
	}

	for _, vision := range entries {
		sight := vision.Sight()

		if sight == "" {
			return nil, fmt.Errorf("a Vision cannot have an empty Sight")
		}

		if _, exists := registry.visions[sight]; exists {
			return nil, fmt.Errorf("Vision already registered: %s", sight)
		}

		registry.visions[sight] = vision
		registry.order = append(registry.order, sight)
	}

	sort.Strings(registry.order)

	return registry, nil
}

func (r *Registry) BeholdAll(
	ctx context.Context,
) ([]*proto.Vision, error) {
	revelations := make([]*proto.Vision, 0, len(r.order))

	for _, sight := range r.order {
		vision := r.visions[sight]

		awake, slumberReason, err := vision.Behold(ctx)
		if err != nil {
			return nil, fmt.Errorf("behold Vision %s: %w", sight, err)
		}

		revelations = append(revelations, &proto.Vision{
			Vision:        vision.Sight(),
			Form:          vision.Form(),
			Awake:         awake,
			SlumberReason: slumberReason,
		})
	}

	return revelations, nil
}

func (r *Registry) Recall(sight string) (Vision, bool) {
	vision, exists := r.visions[sight]
	return vision, exists
}

func (r *Registry) DiscernGaze(
	ctx context.Context,
	gaze *proto.Gaze,
) error {
	if gaze == nil {
		return fmt.Errorf("received an empty Gaze")
	}

	sigil := strings.TrimSpace(gaze.GetSigil())
	if sigil == "" {
		return fmt.Errorf("Gaze has no Sigil")
	}

	if gaze.GetTurn() < 1 {
		return fmt.Errorf(
			"Gaze %s has an invalid turn",
			sigil,
		)
	}

	sight := strings.TrimSpace(gaze.GetVision())
	if sight == "" {
		return fmt.Errorf(
			"Gaze %s has no Vision",
			sigil,
		)
	}

	vision, exists := r.Recall(sight)
	if !exists {
		return fmt.Errorf(
			"Eye has not beheld Vision %s",
			sight,
		)
	}

	if vision.Form() != gaze.GetForm() {
		return fmt.Errorf(
			"Vision %s knows form %d, not form %d",
			sight,
			vision.Form(),
			gaze.GetForm(),
		)
	}

	awake, slumberReason, err := vision.Behold(ctx)
	if err != nil {
		return fmt.Errorf(
			"behold Vision %s before Gaze: %w",
			sight, err,
		)
	}

	if gaze.GetAwake() && !awake {
		return fmt.Errorf(
			"Vision %s is slumbering: %s",
			sight,
			slumberReason,
		)
	}

	if err := vision.DiscernFocus(gaze.GetFocus()); err != nil {
		return fmt.Errorf(
			"Gaze %s focus is unclear: %w",
			sigil,
			err,
		)
	}

	return nil
}