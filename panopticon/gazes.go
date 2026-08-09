package main

import (
	"fmt"
	"strings"
	"time"
)

func (s *PanoptesServer) bestowGaze(
	gaze GazeRecord,
) (GazeRecord, error) {
	gaze.EyeID = strings.TrimSpace(gaze.EyeID)
	gaze.Sight = strings.TrimSpace(gaze.Sight)
	gaze.Sigil = strings.TrimSpace(gaze.Sigil)

	vision, revealed, err := s.chronicle.RecallVision(
		gaze.EyeID,
		gaze.Sight,
	)
	if err != nil {
		return GazeRecord{}, fmt.Errorf(
			"recall Eye Vision before bestowing Gaze: %w",
			err,
		)
	}

	if !revealed {
		return GazeRecord{}, fmt.Errorf(
			"Eye %s has not revealed Vision %s",
			gaze.EyeID,
			gaze.Sight,
		)
	}

	if vision.Form != gaze.Form {
		return GazeRecord{}, fmt.Errorf(
			"Eye %s knows Vision %s form %d, not form %d",
			gaze.EyeID,
			gaze.Sight,
			vision.Form,
			gaze.Form,
		)
	}

	if gaze.Awake && !vision.Awake {
		return GazeRecord{}, fmt.Errorf(
			"Vision %s is slumbering: %s",
			gaze.Sight,
			vision.SlumberReason,
		)
	}

	return s.chronicle.EngraveGaze(
		gaze,
		time.Now().UTC(),
	)
}