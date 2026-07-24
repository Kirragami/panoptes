package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const eyeIDFile = "eye-id"

func loadOrCreateEyeID(stateDir string) (string, error) {
	idPath := filepath.Join(stateDir, eyeIDFile)

	data, err := os.ReadFile(idPath)
	if err == nil {
		eyeID := strings.TrimSpace(string(data))
		if eyeID == "" {
			return "", fmt.Errorf("stored eye ID is empty: %s", idPath)
		}

		return eyeID, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read eye ID: %w", err)
	}

	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return "", fmt.Errorf("create Eye state directory: %w", err)
	}

	eyeID, err := generateEyeID()
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(idPath, []byte(eyeID+"\n"), 0600); err != nil {
		return "", fmt.Errorf("write eye ID: %w", err)
	}

	return eyeID, nil
}

func generateEyeID() (string, error) {
	bytes := make([]byte, 16)

	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random Eye ID: %w", err)
	}

	return hex.EncodeToString(bytes), nil
}
