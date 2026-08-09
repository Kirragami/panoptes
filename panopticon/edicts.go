package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const edictAuthorization = "authorization"

func (s *PanoptesServer) authenticateEdict(
	ctx context.Context,
) error {
	headers := metadata.ValueFromIncomingContext(
		ctx,
		edictAuthorization,
	)

	if len(headers) != 1 {
		return status.Error(
			codes.Unauthenticated,
			"an Edict token is required",
		)
	}

	const bearer = "Bearer "

	credential := strings.TrimSpace(headers[0])
	if !strings.HasPrefix(credential, bearer) {
		return status.Error(
			codes.Unauthenticated,
			"Edict authorization must use Bearer",
		)
	}

	providedToken := strings.TrimSpace(
		strings.TrimPrefix(credential, bearer),
	)

	expectedHash := sha256.Sum256([]byte(s.edictToken))
	providedHash := sha256.Sum256([]byte(providedToken))

	if subtle.ConstantTimeCompare(
		expectedHash[:],
		providedHash[:],
	) != 1 {
		return status.Error(
			codes.PermissionDenied,
			"the Edict token is not accepted",
		)
	}

	return nil
}

func revealGaze(gaze GazeRecord) *proto.Gaze {
	return &proto.Gaze{
		Sigil:  gaze.Sigil,
		Turn:   gaze.Turn,
		Awake:  gaze.Awake,
		Vision: gaze.Sight,
		Form:   gaze.Form,
		Focus:  gaze.Focus,
	}
}

func (s *PanoptesServer) BestowGaze(
	ctx context.Context,
	request *proto.BestowGazeRequest,
) (*proto.BestowGazeResponse, error) {
	if err := s.authenticateEdict(ctx); err != nil {
		return nil, err
	}

	gaze := request.GetGaze()
	if gaze == nil {
		return nil, status.Error(
			codes.InvalidArgument,
			"an Edict must carry a Gaze",
		)
	}

	bestowed, err := s.bestowGaze(GazeRecord{
		EyeID: request.GetEyeId(),
		Sigil: gaze.GetSigil(),
		Awake: gaze.GetAwake(),
		Sight: gaze.GetVision(),
		Form:  gaze.GetForm(),
		Focus: gaze.GetFocus(),
	})
	if err != nil {
		return &proto.BestowGazeResponse{
			Success:       false,
			StatusMessage: err.Error(),
		}, nil
	}

	return &proto.BestowGazeResponse{
		Success:       true,
		StatusMessage: "Gaze bestowed",
		Gaze:          revealGaze(bestowed),
	}, nil
}