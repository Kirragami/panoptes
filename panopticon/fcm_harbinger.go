package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// injecting tokens for now to test
type FCMHarbinger struct {
	messenger *messaging.Client
	chronicle *Chronicle
}

func NewFCMHarbinger(
	ctx context.Context,
	credentialsPath string,
	chronicle *Chronicle,
) (*FCMHarbinger, error) {
	credentialsPath = strings.TrimSpace(credentialsPath)

	if credentialsPath == "" {
		return nil, fmt.Errorf("Firebase credentials path is required")
	}

	if chronicle == nil {
		return nil, fmt.Errorf("Chronicle could not be found")
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
		messenger: messenger,
		chronicle: chronicle,
	}, nil
}

func (h *FCMHarbinger) BearOmen(
	ctx context.Context,
	omen OmenRecord,
) error {
	tokens, err := h.chronicle.RecallOracleTokens()
	if err != nil {
		return err
	}

	if len(tokens) == 0 {
		return nil
	}

	epithet, err := h.chronicle.RecallEpithet(omen.EyeID)
	if err != nil {
		return err
	}

	var failures []error

	for _, token := range tokens {
		_, err := h.messenger.Send(
			ctx,
			&messaging.Message{
				Token: token,
				Data: map[string]string{
					"omen_id":          omen.OmenID,
					"eye_id":           omen.EyeID,
					"epithet":          epithet,
					"gaze_sigil":       omen.GazeSigil,
					"gaze_turn":        strconv.FormatUint(omen.GazeTurn, 10),
					"befallen_at_unix": strconv.FormatInt(omen.BefallenAt.Unix(), 10),
				},
				Android: &messaging.AndroidConfig{
					Priority: "high",
				},
			},
		)
		if err != nil {
			failures = append(
				failures,
				fmt.Errorf("send to Oracle token: %w", err),
			)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf(
			"bear Omen through FCM: %w",
			errors.Join(failures...),
		)
	}

	return nil
}
