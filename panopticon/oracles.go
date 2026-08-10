package main

import (
	"fmt"
	"strings"
	"time"
)

const oracleSealTTL = 15 * time.Minute

func forgeOracleBrand() (string, error) {
	return forgeBrand()
}

func hashOracleBrand(brand string) string {
	return hashBrand(brand)
}

func (s *PanoptesServer) issueOracleSeal() (
	string,
	time.Time,
	error,
) {
	seal, err := forgeSeal()
	if err != nil {
		return "", time.Time{}, err
	}

	createdAt := time.Now().UTC()
	expiresAt := createdAt.Add(oracleSealTTL)

	if err := s.chronicle.InscribeOracleSeal(
		hashSeal(seal),
		createdAt,
		expiresAt,
	); err != nil {
		return "", time.Time{}, err
	}

	return seal, expiresAt, nil
}

func (s *PanoptesServer) pairOracle(
	oracleID string,
	oracleSeal string,
	fcmToken string,
) (string, error) {
	oracleID = strings.TrimSpace(oracleID)
	oracleSeal = strings.TrimSpace(oracleSeal)
	fcmToken = strings.TrimSpace(fcmToken)

	if oracleID == "" {
		return "", fmt.Errorf("Oracle has no identity")
	}

	if oracleSeal == "" {
		return "", fmt.Errorf("Oracle pairing requires a Seal")
	}

	if fcmToken == "" {
		return "", fmt.Errorf("Oracle pairing requires an FCM token")
	}

	brand, err := forgeOracleBrand()
	if err != nil {
		return "", err
	}

	if err := s.chronicle.PairOracle(
		oracleID,
		hashSeal(oracleSeal),
		hashOracleBrand(brand),
		fcmToken,
		time.Now().UTC(),
	); err != nil {
		return "", err
	}

	return brand, nil
}

func (s *PanoptesServer) recognizeOracle(
	oracleID string,
	oracleBrand string,
) error {
	oracleID = strings.TrimSpace(oracleID)

	if oracleID == "" {
		return fmt.Errorf("Oracle has no identity")
	}

	brandHash, paired, err := s.chronicle.RecallOracleBrandHash(
		oracleID,
	)
	if err != nil {
		return fmt.Errorf("recall Oracle Brand: %w", err)
	}

	if !paired || !matchesBrand(brandHash, oracleBrand) {
		return fmt.Errorf("Oracle does not carry a valid Brand")
	}

	return nil
}

func (s *PanoptesServer) refreshOracleToken(
	oracleID string,
	oracleBrand string,
	fcmToken string,
) error {
	if err := s.recognizeOracle(oracleID, oracleBrand); err != nil {
		return err
	}

	return s.chronicle.RefreshOracleToken(
		oracleID,
		fcmToken,
		time.Now().UTC(),
	)
}