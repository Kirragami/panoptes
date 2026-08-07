package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/Kirragami/panoptes/proto"
)

func (s *PanoptesServer) rememberRevelations(
	eyeID string,
	revelations []*proto.Vision,
) error {
	witnessedAt := time.Now().UTC()

	remembered := make(
		[]VisionRecord,
		0,
		len(revelations),
	)

	seen := make(map[string]struct{})

	for _, revelation := range revelations {
		if revelation == nil {
			return fmt.Errorf("received an empty Vision")
		}

		sight := strings.TrimSpace(revelation.GetVision())
		if sight == "" {
			return fmt.Errorf("Vision has an empty Sight")
		}

		if revelation.GetForm() == 0 {
			return fmt.Errorf(
				"Vision %s has an invalid form",
				sight,
			)
		}

		if _, duplicate := seen[sight]; duplicate {
			return fmt.Errorf(
				"Vision was revealed twice: %s",
				sight,
			)
		}
		seen[sight] = struct{}{}

		slumberReason := strings.TrimSpace(
			revelation.GetSlumberReason(),
		)

		if !revelation.GetAwake() && slumberReason == "" {
			return fmt.Errorf(
				"slumbering Vision %s has no reason",
				sight,
			)
		}

		if revelation.GetAwake() {
			slumberReason = ""
		}

		remembered = append(remembered, VisionRecord{
			Sight:         sight,
			Form:          revelation.GetForm(),
			Awake:         revelation.GetAwake(),
			SlumberReason: slumberReason,
			BeheldAt:      witnessedAt,           
		})
	}

	return s.chronicle.RememberVisions(
		eyeID,
		remembered,
		witnessedAt,
	)
}