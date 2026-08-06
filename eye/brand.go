package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// handle the paths gracefully later,
// i swear im just so bored to code the boring stuff rn
const brandFile = "brand"

func recallBrand(stateDir string) (string, error) {
	brandPath := filepath.Join(stateDir, brandFile)

	data, err := os.ReadFile(brandPath)
	if err == nil {
		brand := strings.TrimSpace(string(data))
		if brand == "" {
			return "", fmt.Errorf("stored Brand is empty: %s", brandPath)
		}

		return brand, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}

	return "", fmt.Errorf("read Brand: %w", err)
}

func imprintBrand(stateDir, brand string) error {
	brand = strings.TrimSpace(brand)
	if brand == "" {
		return fmt.Errorf("cannot preserve an empty Brand")
	}

	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return fmt.Errorf("create Eye state directory: %w", err)
	}

	tempFile, err := os.CreateTemp(stateDir, ".brand-*")
	if err != nil {
		return fmt.Errorf("create temporary Brand file: %w", err)
	}

	tempPath := tempFile.Name()

	if err := tempFile.Chmod(0600); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("protect temporary Brand file: %w", err)
	}

	if _, err := tempFile.WriteString(brand + "\n"); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("write Brand: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close Brand file: %w", err)
	}

	brandPath := filepath.Join(stateDir, brandFile)
	if err := os.Rename(tempPath, brandPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("preserve Brand: %w", err)
	}

	return nil
}
