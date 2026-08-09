package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *PanoptesServer) RaiseOmen(
	ctx context.Context,
	omen *proto.Omen,
) (*proto.OmenReceipt, error) {
	if omen == nil {
		return nil, status.Error(
			codes.InvalidArgument,
			"an Omen is required",
		)
	}

	omenID := strings.TrimSpace(omen.GetOmenId())
	if omenID == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"an Omen requires an identity",
		)
	}

	if omen.GetBefallenAtUnix() < 1 {
		return nil, status.Error(
			codes.InvalidArgument,
			"an Omen requires its time of befell",
		)
	}

	if err := s.recognizeEye(
		omen.GetEyeId(),
		omen.GetBrand(),
	); err != nil {
		log.Printf(
			"[PANOPTICON] Omen refused from Eye %s: %v",
			omen.GetEyeId(),
			err,
		)

		return nil, status.Error(
			codes.Unauthenticated,
			"valid Brand is required to raise an Omen",
		)
	}

	gaze, found, err := s.chronicle.RecallGaze(
		omen.GetEyeId(),
		omen.GetGazeSigil(),
	)
	if err != nil {
		return nil, status.Error(
			codes.Internal,
			"Panopticon could not recall the Gaze",
		)
	}

	if !found {
		return &proto.OmenReceipt{
			Received: false,
			Reason:   "the Gaze no longer exists",
		}, nil
	}

	if gaze.Turn != omen.GetGazeTurn() {
		return &proto.OmenReceipt{
			Received: false,
			Reason:   "the Omen belongs to an old Gaze turn",
		}, nil
	}

	if !gaze.Awake {
		return &proto.OmenReceipt{
			Received: false,
			Reason:   "the Gaze is asleep",
		}, nil
	}

	receivedOmen := OmenRecord{
		OmenID:    omen.GetOmenId(),
		EyeID:     omen.GetEyeId(),
		GazeSigil: omen.GetGazeSigil(),
		GazeTurn:  omen.GetGazeTurn(),
		BefallenAt: time.Unix(
			omen.GetBefallenAtUnix(),
			0,
		).UTC(),
		ReceivedAt: time.Now().UTC(),
	}

	isNew, err := s.chronicle.ReceiveOmen(receivedOmen)
	if err != nil {
		return nil, status.Error(
			codes.Internal,
			"Panopticon could not receive the Omen",
		)
	}

	if !isNew {
		return &proto.OmenReceipt{
			Received: true,
			Reason:   "Omen was already received",
		}, nil
	}

	if err := s.harbinger.BearOmen(ctx, receivedOmen); err != nil {
		log.Printf(
			"[PANOPTICON] Harbinger failed for Omen %s: %v",
			receivedOmen.OmenID,
			err,
		)
	}

	log.Printf(
		"[PANOPTICON] An Omen has befallen upon %s from Eye %s",
		gaze.Sigil,
		omen.GetEyeId(),
	)

	return &proto.OmenReceipt{
		Received: true,
		Reason:   "Omen received",
	}, nil
}
