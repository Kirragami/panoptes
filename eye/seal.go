package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const sealFile = "seal"

func recallSeal(stateDir string) (string, error) {
	sealPath := filepath.Join(stateDir, sealFile)

	data, err := os.ReadFile(sealPath)
	if err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}

	return "", fmt.Errorf("read Seal: %w", err)
}

func discardSeal(stateDir string) error {
	err := os.Remove(filepath.Join(stateDir, sealFile))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return fmt.Errorf("discard Seal: %w", err)
}
