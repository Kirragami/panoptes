package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Kirragami/panoptes/proto"
)

func forgeOmenID() (string, error) {
	bytes := make([]byte, 16)

	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("forge Omen identity: %w", err)
	}

	return hex.EncodeToString(bytes), nil
}

func foretellOmen(
	iris *Iris,
	eyeID string,
	gaze *proto.Gaze,
) (*proto.Omen, error) {
	if gaze == nil {
		return nil, fmt.Errorf("cannot foretell an Omen without a Gaze")
	}

	omenID, err := forgeOmenID()
	if err != nil {
		return nil, err
	}

	return &proto.Omen{
		OmenId:         omenID,
		EyeId:          eyeID,
		Brand:          iris.Brand,
		GazeSigil:      gaze.GetSigil(),
		GazeTurn:       gaze.GetTurn(),
		BefallenAtUnix: time.Now().Unix(),
	}, nil
}

func raiseOmen(
	ctx context.Context,
	iris *Iris,
	omen *proto.Omen,
) error {
	if omen == nil {
		return fmt.Errorf("cannot raise an empty Omen")
	}

	connection, err := openPanopticon(iris)
	if err != nil {
		return err
	}
	defer connection.Close()

	panopticon := proto.NewPanoptesOmenServiceClient(connection)

	receipt, err := panopticon.RaiseOmen(ctx, omen)
	if err != nil {
		return fmt.Errorf("raise Omen: %w", err)
	}

	if !receipt.GetReceived() {
		return fmt.Errorf(
			"Panopticon did not receive Omen: %s",
			receipt.GetReason(),
		)
	}

	return nil
}