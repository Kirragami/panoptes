package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	// "github.com/Kirragami/panoptes/proto"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	_ "modernc.org/sqlite"
)

type Sighting struct {
	EyeID     string
	FirstSeen time.Time
	LastSeen  time.Time
}

type VisionRecord struct {
	Sight         string
	Form          uint32
	Awake         bool
	SlumberReason string
	BeheldAt      time.Time
}

type GazeRecord struct {
	EyeID string
	Sigil string
	Turn  uint64
	Awake bool
	Sight string
	Form  uint32
	Focus *structpb.Struct
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

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS eye_visions (
			eye_id TEXT NOT NULL,
			sight TEXT NOT NULL,
			form INTEGER NOT NULL,
			awake INTEGER NOT NULL,
			slumber_reason TEXT NOT NULL,
			beheld_at_unix INTEGER NOT NULL,
			PRIMARY KEY (eye_id, sight)
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Chronicle Visions: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS eye_gazes (
			eye_id TEXT NOT NULL,
			sigil TEXT NOT NULL,
			turn INTEGER NOT NULL,
			awake INTEGER NOT NULL,
			sight TEXT NOT NULL,
			form INTEGER NOT NULL,
			focus_json TEXT NOT NULL,
			updated_at_unix INTEGER NOT NULL,
			PRIMARY KEY (eye_id, sigil)
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Gaze Chronicles: %w", err)
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

func (c *Chronicle) RememberVisions(
	eyeID string,
	visions []VisionRecord,
	beheldAt time.Time,
) error {
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin Vision remembrance: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.Exec(
		`DELETE FROM eye_visions WHERE eye_id = ?`,
		eyeID,
	)
	if err != nil {
		return fmt.Errorf("forget old Visions: %w", err)
	}

	for _, vision := range visions {
		_, err = tx.Exec(
			`INSERT INTO eye_visions (
					eye_id,
					sight,
					form,
					awake,
					slumber_reason,
					beheld_at_unix
				) VALUES (?, ?, ?, ?, ?, ?)`,
			eyeID,
			vision.Sight,
			vision.Form,
			vision.Awake,
			vision.SlumberReason,
			beheldAt.Unix(),
		)
		if err != nil {
			return fmt.Errorf(
				"remember Vision %s: %w",
				vision.Sight,
				err,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Vision remembrance: %w", err)
	}

	return nil
}

func (c *Chronicle) EngraveGaze(
	gaze GazeRecord,
	engravedAt time.Time,
) (GazeRecord, error) {
	gaze.EyeID = strings.TrimSpace(gaze.EyeID)
	gaze.Sigil = strings.TrimSpace(gaze.Sigil)
	gaze.Sight = strings.TrimSpace(gaze.Sight)

	if gaze.EyeID == "" {
		return GazeRecord{}, fmt.Errorf("Gaze is from a dead Eye")
	}

	if gaze.Sigil == "" {
		return GazeRecord{}, fmt.Errorf("Gaze has no Sigil")
	}

	if gaze.Sight == "" {
		return GazeRecord{}, fmt.Errorf("Gaze %s has no Sight", gaze.Sigil)
	}

	if gaze.Form == 0 {
		return GazeRecord{}, fmt.Errorf(
			"Gaze %s has an invalid form",
			gaze.Sigil,
		)
	}

	if gaze.Focus == nil {
		gaze.Focus = &structpb.Struct{}
	}

	focusJSON, err := protojson.Marshal(gaze.Focus)
	if err != nil {
		return GazeRecord{}, fmt.Errorf(
			"shape Gaze %s focus: %w",
			gaze.Sigil,
			err,
		)
	}

	tx, err := c.db.Begin()
	if err != nil {
		return GazeRecord{}, fmt.Errorf("begin Gaze engraving: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var previousTurn uint64

	err = tx.QueryRow(
		`SELECT turn
		 FROM eye_gazes
	     WHERE eye_id = ? AND sigil = ?`,
		gaze.EyeID,
		gaze.Sigil,
	).Scan(&previousTurn)

	if err == sql.ErrNoRows {
		gaze.Turn = 1
	} else if err != nil {
		return GazeRecord{}, fmt.Errorf(
			"recall previous Gaze turn: %w",
			err,
		)
	} else {
		gaze.Turn = previousTurn + 1
	}

	_, err = tx.Exec(
		`INSERT INTO eye_gazes (
			eye_id,
			sigil,
			turn,
			awake,
			sight,
			form,
			focus_json,
			updated_at_unix	
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(eye_id, sigil) DO UPDATE SET
			turn = excluded.turn,
			awake = excluded.awake,
			sight = excluded.sight,
			form = excluded.form,
			focus_json = excluded.focus_json,
			updated_at_unix = excluded.updated_at_unix`,
		gaze.EyeID,
		gaze.Sigil,
		gaze.Turn,
		gaze.Awake,
		gaze.Sight,
		gaze.Form,
		string(focusJSON),
		engravedAt.Unix(),
	)
	if err != nil {
		return GazeRecord{}, fmt.Errorf(
			"engrave Gaze %s: %w",
			gaze.Sigil,
			err,
		)
	}
	if err := tx.Commit(); err != nil {
		return GazeRecord{}, fmt.Errorf(
			"commit Gaze engraving: %w",
			err,
		)
	}

	return gaze, nil
}

func (c *Chronicle) RecallGazes(
	eyeID string,
) ([]GazeRecord, error) {
	eyeID = strings.TrimSpace(eyeID)
	if eyeID == "" {
		return nil, fmt.Errorf("cannot recall Gazes for an empty Eye")
	}

	rows, err := c.db.Query(
		`SELECT
			sigil,
			turn,
			awake,
			sight,
			form,
			focus_json
		FROM eye_gazes
		WHERE eye_id = ?
		ORDER BY sigil ASC`,
		eyeID,
	)
	if err != nil {
		return nil, fmt.Errorf("recall Gazes: %w", err)
	}
	defer rows.Close()

	var gazes []GazeRecord

	for rows.Next() {
		var gaze GazeRecord
		var turn int64
		var awake int64
		var form int64
		var focusJSON string

		if err := rows.Scan(
			&gaze.Sigil,
			&turn,
			&awake,
			&gaze.Sight,
			&form,
			&focusJSON,
		); err != nil {
			return nil, fmt.Errorf("read Gaze: %w", err)
		}

		if turn < 1 {
			return nil, fmt.Errorf(
				"Gaze %s has an invalid turn",
				gaze.Sigil,
			)
		}

		if form < 1 {
			return nil, fmt.Errorf(
				"Gaze %s has an invalid form",
				gaze.Sigil,
			)
		}

		focus := &structpb.Struct{}
		if err :=protojson.Unmarshal(
			[]byte(focusJSON),
			focus,
		); err != nil {
			return nil, fmt.Errorf(
				"shape recalled Gaze %s focus: %w",
				gaze.Sigil,
				err,
			)
		}

		gaze.EyeID = eyeID
		gaze.Turn = uint64(turn)
		gaze.Awake = awake != 0
		gaze.Form = uint32(form)
		gaze.Focus = focus

		gazes = append(gazes, gaze)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Gazes: %w", err)
	}

	return gazes, nil
}
func (c *Chronicle) RecallVision(
	eyeID string,
	sight string,
) (VisionRecord, bool, error) {
	eyeID = strings.TrimSpace(eyeID)
	sight = strings.TrimSpace(sight)

	if eyeID == "" {
		return VisionRecord{}, false, fmt.Errorf(
			"cannot recall Vision for an empty Eye",
		)
	}

	if sight == "" {
		return VisionRecord{}, false, fmt.Errorf(
			"cannot recall an empty Sight",
		)
	}

	var vision VisionRecord
	var form int64
	var awake int64
	var beheldAtUnix int64

	err := c.db.QueryRow(
		`SELECT 
			form,
			awake,
			slumber_reason,
			beheld_at_unix
		FROM eye_visions
		WHERE eye_id = ? AND sight = ?`,
		eyeID,
		sight,
	).Scan(
		&form,
		&awake,
		&vision.SlumberReason,
		&beheldAtUnix,
	)
	if err == sql.ErrNoRows {
		return VisionRecord{}, false, nil
	}

	if err != nil {
		return VisionRecord{}, false, fmt.Errorf(
			"recall Vision %s: %w",
			sight,
			err,
		)
	}

	if form < 1 {
		return VisionRecord{}, false, fmt.Errorf(
			"Vision %s has an invalid form",
			sight,
		)
	}

	vision.Sight = sight
	vision.Form = uint32(form)
	vision.Awake = awake != 0
	vision.BeheldAt = time.Unix(beheldAtUnix, 0).UTC()

	return vision, true, nil
}