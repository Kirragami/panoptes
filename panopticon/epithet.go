package main

import (
	"fmt"
	"regexp"
	"strings"
)

var epithetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func normalizeEpithet(value string) (string, error) {
	epithet := strings.TrimSpace(value)
	if epithet == "" {
		return "", fmt.Errorf("Epithet is required")
	}
	if !epithetPattern.MatchString(epithet) {
		return "", fmt.Errorf(
			"Epithet may only contain letter, digits, dots, hyphens, and underscores",
		)
	}

	return epithet, nil
}
