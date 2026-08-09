package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// injecting tokens for now to test
type FCMHarbinger struct {
	messenger   *messaging.Client
	deviceToken string
}

func NewFCMHarbinger(
	ctx context.Context,
	credentialsPath string,
	deviceToken string,
) (*FCMHarbinger, error) {
	credentialsPath = strings.TrimSpace(credentialsPath)
	deviceToken = strings.TrimSpace(deviceToken)

	if credentialsPath == "" {
		return nil, fmt.Errorf("Firebase credentials path is required")
	}

	if deviceToken == "" {
		return nil, fmt.Errorf("FCM Device Token is required")
	}

	app, err := firebase.NewApp(
		ctx,
		nil,
		option.WithCredentialsFile(credentialsPath),
	)
	if err != nil {
		return nil, fmt.Errorf("awaken Firebase: %w", err)
	}

	messenger, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("awaken FCM messenger: %w", err)
	}

	return &FCMHarbinger{
		messenger:   messenger,
		deviceToken: deviceToken,
	}, nil
}

func (h *FCMHarbinger) BearOmen(
	ctx context.Context,
	omen OmenRecord,
) error {
	_, err := h.messenger.Send(
		ctx,
		&messaging.Message{
			Token: h.deviceToken,
			Data: map[string]string{
				"omen_id": omen.OmenID,
				"eye_id": omen.EyeID,
				"gaze_sigil": omen.GazeSigil,
				"gaze_turn": strconv.FormatUint(omen.GazeTurn, 10),
				"befallen_at_unix": strconv.FormatInt(omen.BefallenAt.Unix(), 10),
			},
			Android: &messaging.AndroidConfig{
				Priority: "high",
			},
		},
	)
	if err != nil {
		return fmt.Errorf(
			"bear Omen %s through FCM: %w",
			omen.OmenID,
			err,
		)
	}

	return nil
}