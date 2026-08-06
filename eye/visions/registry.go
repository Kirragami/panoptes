package visions

import (
	"context"
	"fmt"
	"sort"

	"github.com/Kirragami/panoptes/proto"
)

// lets just go with this design from start for better scalability teehee
type Vision interface {
	Sight() string
	Form() uint32
	Behold(context.Context) (awake bool, slumberReason string, err error)
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
