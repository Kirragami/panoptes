package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const sealTTL = 15 * time.Minute

func forgeSeal() (string, error) {
	bytes := make([]byte, 32)

	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("forge Seal: %w", err)
	}

	return hex.EncodeToString(bytes), nil
}

func (s *PanoptesServer) issueSeal() (string, time.Time, error) {
	seal, err := forgeSeal()
	if err != nil {
		return "", time.Time{}, err
	}

	createdAt := time.Now().UTC()
	expiresAt := createdAt.Add(sealTTL)

	// need to store in a better way bro cmon :'))
	if err := s.chronicle.InscribeSeal(seal, createdAt, expiresAt); err != nil {
		return "", time.Time{}, err
	}

	return seal, expiresAt, nil
}

func (s *PanoptesServer) admitBind(eyeID, seal, brand string) error {
	brandHash, branded, err := s.chronicle.RecallBrandHash(eyeID)
	if err != nil {
		return fmt.Errorf("recall Eye brand: %w", err)
	}

	if branded {
		if !matchesBrand(brandHash, brand) {
			return fmt.Errorf("valid Brand is required to Bind this Eye")
		}

		return nil
	}

	seal = strings.TrimSpace(seal)
	if seal == "" {
		return fmt.Errorf("Seal is required to Bind a new Eye")
	}

	if err := s.chronicle.ConsumeSeal(seal, eyeID, time.Now().UTC()); err != nil {
		return err
	}

	return nil
}
