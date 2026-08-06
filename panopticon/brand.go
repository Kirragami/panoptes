package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"fmt"
	"time"
)

func forgeBrand() (string, error) {
	bytes := make([]byte, 32)

	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("forge Brand %w", err)
	}

	return hex.EncodeToString(bytes), nil
}

func hashBrand(brand string) string {
	sum := sha256.Sum256([]byte(brand))
	return hex.EncodeToString(sum[:])
}

func (s *PanoptesServer) brandEye(eyeID string) (string, error) {
	_, exists, err := s.chronicle.RecallBrandHash(eyeID)
	if err != nil {
		return "", err
	}
	if exists {
		return "", nil
	}

	brand, err := forgeBrand()
	if err != nil {
		return "", err
	}

	if err := s.chronicle.InscribeBrand(
		eyeID,
		hashBrand(brand),
		time.Now().UTC(),
	); err != nil {
		return "", err
	}

	return brand, nil
}

func matchesBrand(brandHash, brand string) bool {
	brand = strings.TrimSpace(brand)
	if brand == "" {
		return false
	}

	providedHash := hashBrand(brand)

	return subtle.ConstantTimeCompare(
		[]byte(brandHash),
		[]byte(providedHash),
	) == 1
}