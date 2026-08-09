package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"fmt"
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

func (s *PanoptesServer) recognizeEye(
	eyeID string,
	brand string,
) error {
	eyeID = strings.TrimSpace(eyeID)

	if eyeID == "" {
		return fmt.Errorf("Eye has no identity")
	}

	brandHash, branded, err := s.chronicle.RecallBrandHash(eyeID)
	if err != nil {
		return fmt.Errorf("recall Eye Brand: %w", err)
	}

	if !branded || !matchesBrand(brandHash, brand) {
		return fmt.Errorf("Eye does not carry a valid Brand")
	}

	return nil
}