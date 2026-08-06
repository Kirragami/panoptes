package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Sighting struct {
	EyeID     string
	FirstSeen time.Time
	LastSeen  time.Time
}

type Chronicle struct {
	db *sql.DB
}

func openChronicle(path string) (*Chronicle, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open Chronicle: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping Chronicle: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sightings (
			eye_id TEXT PRIMARY KEY,
			first_seen_unix INTEGER NOT NULL,
			last_seen_unix INTEGER NOT NULL
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Chronicle sightings: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS seals (
			seal_hash TEXT PRIMARY KEY,
			created_at_unix INTEGER NOT NULL,
			expires_at_unix INTEGER NOT NULL,
			consumed_at_unix INTEGER,
			bound_eye_id TEXT
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Chronicle seals: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brands (
			eye_id TEXT PRIMARY KEY,
			brand_hash TEXT NOT NULL,
			created_at_unix INTEGER NOT NULL
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Chronicle brands: %w", err)
	}

	return &Chronicle{db: db}, nil
}

func (c *Chronicle) Close() error {
	return c.db.Close()
}

func (c *Chronicle) RecordSight(eyeID string, seenAt time.Time) (Sighting, bool, error) {
	seenUnix := seenAt.Unix()

	var existing Sighting
	var firstSeenUnix, lastSeenUnix int64

	err := c.db.QueryRow(
		`SELECT eye_id, first_seen_unix, last_seen_unix
		 FROM sightings
		 WHERE eye_id = ?`,
		eyeID,
	).Scan(&existing.EyeID, &firstSeenUnix, &lastSeenUnix)

	if err == sql.ErrNoRows {
		_, err = c.db.Exec(
			`INSERT INTO sightings (eye_id, first_seen_unix, last_seen_unix)
			 VALUES (?, ?, ?)`,
			eyeID,
			seenUnix,
			seenUnix,
		)
		if err != nil {
			return Sighting{}, false, fmt.Errorf("inscribe first sighting: %w", err)
		}

		return Sighting{
			EyeID:     eyeID,
			FirstSeen: seenAt,
			LastSeen:  seenAt,
		}, true, nil
	}

	if err != nil {
		return Sighting{}, false, fmt.Errorf("consult Chronicle: %w", err)
	}

	_, err = c.db.Exec(
		`UPDATE sightings
		 SET last_seen_unix = ?
		 WHERE eye_id = ?`,
		seenUnix,
		eyeID,
	)
	if err != nil {
		return Sighting{}, false, fmt.Errorf("update sighting: %w", err)
	}

	return Sighting{
		EyeID:     existing.EyeID,
		FirstSeen: time.Unix(firstSeenUnix, 0).UTC(),
		LastSeen:  seenAt.UTC(),
	}, false, nil
}

func (c *Chronicle) RecallSightings() ([]Sighting, error) {
	rows, err := c.db.Query(
		`SELECT eye_id, first_seen_unix, last_seen_unix
		 FROM sightings
		 ORDER BY first_seen_unix ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("recall sightings: %w", err)
	}
	defer rows.Close()

	var sightings []Sighting

	for rows.Next() {
		var sighting Sighting
		var firstSeenUnix, lastSeenUnix int64

		if err := rows.Scan(
			&sighting.EyeID,
			&firstSeenUnix,
			&lastSeenUnix,
		); err != nil {
			return nil, fmt.Errorf("read sighting: %w", err)
		}

		sighting.FirstSeen = time.Unix(firstSeenUnix, 0).UTC()
		sighting.LastSeen = time.Unix(lastSeenUnix, 0).UTC()
		sightings = append(sightings, sighting)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sightings: %w", err)
	}

	return sightings, nil
}

func (c *Chronicle) InscribeSeal(sealHash string, createdAt, expiresAt time.Time) error {
	_, err := c.db.Exec(
		`INSERT INTO seals (seal_hash, created_at_unix, expires_at_unix)
		 VALUES (?, ?, ?)`,
		sealHash,
		createdAt.Unix(),
		expiresAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("inscribe Seal: %w", err)
	}

	return nil
}

func (c *Chronicle) RecallBrandHash(eyeID string) (string, bool, error) {
	var brandHash string

	err := c.db.QueryRow(
		`SELECT brand_hash FROM brands WHERE eye_id = ?`,
		eyeID,
	).Scan(&brandHash)

	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("recall Brand: %w", err)
	}

	return brandHash, true, nil
}

func (c *Chronicle) ConsumeSealAndInscribeBrand(
	sealHash string,
	eyeID string,
	brandHash string,
	admittedAt time.Time,
) error {
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin Eye admission: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := tx.Exec(
		`UPDATE seals
		 SET consumed_at_unix = ?, bound_eye_id = ?
		 WHERE seal_hash = ?
		 	AND consumed_at_unix IS NULL
			AND expires_at_unix >= ?`,
		admittedAt.Unix(),
		eyeID,
		sealHash,
		admittedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("consume Seal during admission: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read Seal admission result: %w", err)
	}

	if affected != 1 {
		return fmt.Errorf("Seal is invalid, expired or already consumed")
	}

	_, err = tx.Exec(
		`INSERT INTO brands (eye_id, brand_hash, created_at_unix)
		 VALUES (?, ?, ?)`,
		eyeID,
		brandHash,
		admittedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("inscribe Brand during admission: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Eye admission: %w", err)
	}

	return nil
}
