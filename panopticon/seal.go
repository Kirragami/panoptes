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

	if err := s.chronicle.InscribeSeal(seal, createdAt, expiresAt); err != nil {
		return "", time.Time{}, err
	}

	return seal, expiresAt, nil
}

func (s *PanoptesServer) isKnownEye(eyeID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, known := s.eyes[eyeID]
	return known
}

func (s *PanoptesServer) admitBind(eyeID, seal string) error {
	seal = strings.TrimSpace(seal)

	if s.isKnownEye(eyeID) && seal == "" {
		return nil
	}

	if seal == "" {
		return fmt.Errorf("Seal is required to Bind a new Eye")
	}

	if err := s.chronicle.ConsumeSeal(seal, eyeID, time.Now().UTC()); err != nil {
		return err
	}

	return nil
}