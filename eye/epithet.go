package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const epithetFile = "epithet"

var epithetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func resolveEpithet(stateDir string) (string, error) {
	if env := strings.TrimSpace(os.Getenv("EYE_EPITHET")); env != "" {
		return normalizeEpithet(env)
	}

	stored, err := recallEpithet(stateDir)
	if err != nil {
		return "", err
	}
	if stored != "" {
		return stored, nil
	}

	host, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("behold host Epithet: %w", err)
	}

	epithet, err := normalizeEpithet(host)
	if err != nil {
		return "", fmt.Errorf("set EYE_EPITHET: %w", err)
	}

	return epithet, nil
}

func recallEpithet(stateDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, epithetFile))
	if err == nil {
		return normalizeEpithet(string(data))
	}

	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}

	return "", fmt.Errorf("read Epithet: %w", err)
}

func imprintEpithet(stateDir, epithet string) error {
	epithet, err := normalizeEpithet(epithet)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return fmt.Errorf("create Eye state directory: %w", err)
	}

	if err := os.WriteFile(
		filepath.Join(stateDir, epithetFile),
		[]byte(epithet+"\n"),
		0600,
	); err != nil {
		return fmt.Errorf("write Epithet: %w", err)
	}

	return nil
}

func normalizeEpithet(value string) (string, error) {
	epithet := strings.TrimSpace(value)
	if epithet == "" {
		return "", fmt.Errorf("Epithet is required")
	}
	if !epithetPattern.MatchString(epithet) {
		return "", fmt.Errorf(
			"Epithet may only contain letters, digits, dots, hyphens, and underscores",
		)
	}

	return epithet, nil
}
