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
	Epithet   string
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

type OmenRecord struct {
	OmenID     string
	EyeID      string
	GazeSigil  string
	GazeTurn   uint64
	BefallenAt time.Time
	ReceivedAt time.Time
}

type OracleRecord struct {
	OracleID  string
	PairedAt  time.Time
	RevokedAt *time.Time
}

type SealRecord struct {
	Kind       string
	ForgedAt   time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
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
			eye_epithet TEXT NOT NULL DEFAULT '',
			first_seen_unix INTEGER NOT NULL,
			last_seen_unix INTEGER NOT NULL
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Chronicle sightings: %w", err)
	}

	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS sightings_epithet_unique
		ON sightings (lower(eye_epithet))
		WHERE eye_epithet != ''
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Chronicle Epithet uniqueness: %w", err)
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

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS omens (
			omen_id TEXT PRIMARY KEY,
			eye_id TEXT NOT NULL,
			gaze_sigil TEXT NOT NULL,
			gaze_turn INTEGER NOT NULL,
			befallen_at_unix INTEGER NOT NULL,
			received_at_unix INTEGER NOT NULL
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Chronicle Omens: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS oracle_seals (
			seal_hash TEXT PRIMARY KEY,
			created_at_unix INTEGER NOT NULL,
			expires_at_unix INTEGER NOT NULL,
			consumed_at_unix INTEGER,
			bound_oracle_id TEXT
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Chronicle Oracle Seals: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS oracles (
			oracle_id TEXT PRIMARY KEY,
			brand_hash TEXT NOT NULL,
			paired_at_unix INTEGER NOT NULL,
			revoked_at_unix INTEGER
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Chronicle Oracles: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS oracle_tokens (
			oracle_id TEXT PRIMARY KEY,
			fcm_token TEXT NOT NULL,
			refreshed_at_unix INTEGER NOT NULL
		);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("prepare Chronicle Oracle Tokens: %w", err)
	}

	return &Chronicle{db: db}, nil
}

func (c *Chronicle) Close() error {
	return c.db.Close()
}

func (c *Chronicle) RecordSight(eyeID string, seenAt time.Time) (Sighting, bool, error) {
	seenAtUnix := seenAt.Unix()

	var existing Sighting
	var firstSeenUnix, lastSeenUnix int64

	err := c.db.QueryRow(
		`SELECT eye_id, first_seen_unix, last_seen_unix, eye_epithet
		 FROM sightings
		 WHERE eye_id = ?`,
		 eyeID,
	).Scan(
		&existing.EyeID,
		&firstSeenUnix,
		&lastSeenUnix,
		&existing.Epithet,
	)

	if err == sql.ErrNoRows {
		_, err = c.db.Exec(
			`INSERT INTO sightings (eye_id, first_seen_unix, last_seen_unix)
			 VALUES (?, ?, ?)`,
			 eyeID,
			 seenAtUnix,
			 seenAtUnix,
		)
		if err != nil {
			return Sighting{}, false, fmt.Errorf("inscribe first sighting: %w", err)
		}

		return Sighting{
			EyeID:      eyeID,
			FirstSeen:  seenAt,
			LastSeen:   seenAt,
		}, true, nil
	}

	if err != nil {
		return Sighting{}, false, fmt.Errorf("consult Chronicle: %w", err)
	}

	_, err = c.db.Exec(
		`UPDATE sightings
		 SET last_seen_unix = ?
		 WHERE eye_id = ?`,
		 seenAtUnix,
		 eyeID,
	)
	if err != nil {
		return Sighting{}, false, fmt.Errorf("update sighting: %w", err)
	}

	return Sighting{
		EyeID: existing.EyeID,
		Epithet: existing.Epithet,
		FirstSeen: time.Unix(firstSeenUnix, 0).UTC(),
		LastSeen: seenAt.UTC(),
	}, false, nil
}

func (c *Chronicle) RecallEpithet(eyeID string) (string, error) {
	eyeID = strings.TrimSpace(eyeID)
	if eyeID == "" {
		return "", fmt.Errorf("cannot recall an Epithet for an empty Eye")
	}

	var epithet string
	err := c.db.QueryRow(
		`SELECT eye_epithet FROM sightings
		 WHERE eye_id = ?`,
		eyeID,
	).Scan(&epithet)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("recall Epithet: %w", err)
	}

	return epithet, nil
}

func (c *Chronicle) RememberEpithet(eyeID, epithet string) error {
	eyeID = strings.TrimSpace(eyeID)
	if eyeID == "" {
		return fmt.Errorf("cannot remember an Epithet for an empty Eye")
	}

	epithet, err := normalizeEpithet(epithet)
	if err != nil {
		return err
	}

	var holder string
	err = c.db.QueryRow(
		`SELECT eye_id FROM sightings
		WHERE lower(eye_epithet) = lower(?) AND eye_id != ?`,
		epithet,
		eyeID,
	).Scan(&holder)
	if err == nil {
		return fmt.Errorf("Epithet %s is already borne by another Eye", epithet)
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("consult Epithet: %w", err)
	}

	result, err := c.db.Exec(
		`UPDATE sightings
		SET eye_epithet = ?
		WHERE eye_id = ?`,
		epithet,
		eyeID,
	)
	if err != nil {
		return fmt.Errorf("remember Epithet: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("confirm Epithet remembrance: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("Eye %s has no sighting to bear an Epithet", eyeID)
	}

	return nil
}

func (c *Chronicle) RecallSightings() ([]Sighting, error) {
	rows, err := c.db.Query(
		`SELECT eye_id, first_seen_unix, last_seen_unix, eye_epithet
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
			&sighting.Epithet,
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

func (c *Chronicle) RecallSightingsPage(
	query string,
	limit int,
	offset int,
) ([]Sighting, int, error) {
	query = strings.TrimSpace(query)
	if len(query) > 256 {
		return nil, 0, fmt.Errorf("Eye search query is too long")
	}
	if limit < 1 || limit > 100 {
		return nil, 0, fmt.Errorf("Eye page limit must be between 1 and 100")
	}
	if offset < 0 {
		return nil, 0, fmt.Errorf("Eye page offset cannot be negative")
	}

	whereClause := ""
	var queryArguments []any
	if query != "" {
		whereClause = " WHERE eye_id LIKE ? OR eye_epithet LIKE ?"
		like := "%" + query + "%"
		queryArguments = append(queryArguments, like, like)
	}

	var total int
	countStatement := "SELECT COUNT(*) FROM sightings" + whereClause
	if err := c.db.QueryRow(
		countStatement,
		queryArguments...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sightings: %w", err)
	}

	arguments := append([]any{}, queryArguments...)
	arguments = append(arguments, limit, offset)
	rows, err := c.db.Query(
		`SELECT eye_id, first_seen_unix, last_seen_unix, eye_epithet
		 FROM sightings`+whereClause+`
		 ORDER BY last_seen_unix DESC, eye_id ASC
		 LIMIT ? OFFSET ?`,
		arguments...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("recall sighting page: %w", err)
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
			&sighting.Epithet,
		); err != nil {
			return nil, 0, fmt.Errorf("read sighting: %w", err)
		}

		sighting.FirstSeen = time.Unix(firstSeenUnix, 0).UTC()
		sighting.LastSeen = time.Unix(lastSeenUnix, 0).UTC()
		sightings = append(sightings, sighting)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate sighting page: %w", err)
	}

	return sightings, total, nil
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

func (c *Chronicle) RecallSeals() ([]SealRecord, error) {
	rows, err := c.db.Query(
		`SELECT
			kind,
			forged_at_unix,
			expires_at_unix,
			consumed_at_unix
		FROM (
			SELECT
				'Eye' AS kind,
				created_at_unix AS forged_at_unix,
				expires_at_unix,
				consumed_at_unix
			FROM seals
			UNION ALL
			SELECT
				'Oracle' AS kind,
				created_at_unix AS forged_at_unix,
				expires_at_unix,
				consumed_at_unix
			FROM oracle_seals
		)
		ORDER BY forged_at_unix DESC, kind ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("recall Seals: %w", err)
	}
	defer rows.Close()

	var seals []SealRecord
	for rows.Next() {
		var seal SealRecord
		var forgedAtUnix int64
		var expiresAtUnix int64
		var consumedAtUnix sql.NullInt64

		if err := rows.Scan(
			&seal.Kind,
			&forgedAtUnix,
			&expiresAtUnix,
			&consumedAtUnix,
		); err != nil {
			return nil, fmt.Errorf("read Seal: %w", err)
		}

		seal.ForgedAt = time.Unix(forgedAtUnix, 0).UTC()
		seal.ExpiresAt = time.Unix(expiresAtUnix, 0).UTC()
		if consumedAtUnix.Valid {
			consumedAt := time.Unix(consumedAtUnix.Int64, 0).UTC()
			seal.ConsumedAt = &consumedAt
		}

		seals = append(seals, seal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Seals: %w", err)
	}

	return seals, nil
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
		if err := protojson.Unmarshal(
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

func (c *Chronicle) RecallGaze(
	eyeID string,
	sigil string,
) (GazeRecord, bool, error) {
	eyeID = strings.TrimSpace(eyeID)
	sigil = strings.TrimSpace(sigil)

	if eyeID == "" {
		return GazeRecord{}, false, fmt.Errorf(
			"cannot recall Gaze for an empty Eye",
		)
	}

	if sigil == "" {
		return GazeRecord{}, false, fmt.Errorf(
			"cannot recall a Gaze without Sigil",
		)
	}

	var gaze GazeRecord
	var turn int64
	var awake int64
	var form int64
	var focusJSON string

	err := c.db.QueryRow(
		`SELECT
			turn,
			awake,
			sight,
			form,
			focus_json
		FROM eye_gazes
		WHERE eye_id = ? AND sigil = ?`,
		eyeID,
		sigil,
	).Scan(
		&turn,
		&awake,
		&gaze.Sight,
		&form,
		&focusJSON,
	)

	if err == sql.ErrNoRows {
		return GazeRecord{}, false, nil
	}

	if err != nil {
		return GazeRecord{}, false, fmt.Errorf(
			"recall Gaze %s: %w",
			sigil,
			err,
		)
	}

	if turn < 1 || form < 1 {
		return GazeRecord{}, false, fmt.Errorf(
			"Gaze %s is malformed in Chronicle",
			sigil,
		)
	}

	focus := &structpb.Struct{}
	if err := protojson.Unmarshal(
		[]byte(focusJSON),
		focus,
	); err != nil {
		return GazeRecord{}, false, fmt.Errorf(
			"shape recalled Gaze %s focus: %w",
			sigil,
			err,
		)
	}

	gaze.EyeID = eyeID
	gaze.Sigil = sigil
	gaze.Turn = uint64(turn)
	gaze.Awake = awake != 0
	gaze.Form = uint32(form)
	gaze.Focus = focus

	return gaze, true, nil
}

func (c *Chronicle) RecallVisions(
	eyeID string,
) ([]VisionRecord, error) {
	eyeID = strings.TrimSpace(eyeID)

	if eyeID == "" {
		return nil, fmt.Errorf("cannot recall Visions for an empty Eye")
	}

	rows, err := c.db.Query(
		`SELECT
			sight,
			form,
			awake,
			slumber_reason,
			beheld_at_unix
		FROM eye_visions
		WHERE eye_id = ?
		ORDER BY sight ASC`,
		eyeID,
	)
	if err != nil {
		return nil, fmt.Errorf("recall Visions: %w", err)
	}
	defer rows.Close()

	var visions []VisionRecord

	for rows.Next() {
		var vision VisionRecord
		var form int64
		var awake int64
		var beheldAtUnix int64

		if err := rows.Scan(
			&vision.Sight,
			&form,
			&awake,
			&vision.SlumberReason,
			&beheldAtUnix,
		); err != nil {
			return nil, fmt.Errorf("read Vision: %w", err)
		}

		if form < 1 {
			return nil, fmt.Errorf(
				"Vision %s has an invalid form",
				vision.Sight,
			)
		}

		vision.Form = uint32(form)
		vision.Awake = awake != 0
		vision.BeheldAt = time.Unix(beheldAtUnix, 0).UTC()
		visions = append(visions, vision)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Visions: %w", err)
	}

	return visions, nil
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

func (c *Chronicle) ReceiveOmen(
	omen OmenRecord,
) (bool, error) {
	omen.OmenID = strings.TrimSpace(omen.OmenID)
	omen.EyeID = strings.TrimSpace(omen.EyeID)
	omen.GazeSigil = strings.TrimSpace(omen.GazeSigil)

	if omen.OmenID == "" {
		return false, fmt.Errorf("Omen has no identity")
	}
	if omen.EyeID == "" {
		return false, fmt.Errorf("Omen has no Eye")
	}
	if omen.GazeSigil == "" {
		return false, fmt.Errorf("Omen has no Gaze Sigil")
	}
	if omen.GazeTurn < 1 {
		return false, fmt.Errorf("Omen has an invalid Gaze turn")
	}
	if omen.BefallenAt.IsZero() {
		return false, fmt.Errorf("Omen has no time of befell")
	}
	if omen.ReceivedAt.IsZero() {
		return false, fmt.Errorf("Omen has no received time")
	}

	result, err := c.db.Exec(
		`INSERT INTO omens (
			omen_id,
			eye_id,
			gaze_sigil,
			gaze_turn,
			befallen_at_unix,
			received_at_unix
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(omen_id) DO NOTHING`,
		omen.OmenID,
		omen.EyeID,
		omen.GazeSigil,
		omen.GazeTurn,
		omen.BefallenAt.Unix(),
		omen.ReceivedAt.Unix(),
	)
	if err != nil {
		return false, fmt.Errorf(
			"received Omen %s: %w",
			omen.OmenID,
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf(
			"read Omen receipt: %w",
			err,
		)
	}

	return affected == 1, nil
}

func (c *Chronicle) RecallRecentOmens(
	limit int,
) ([]OmenRecord, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("Omen recall limit must be between 1 and 100")
	}

	rows, err := c.db.Query(
		`SELECT
			omen_id,
			eye_id,
			gaze_sigil,
			gaze_turn,
			befallen_at_unix,
			received_at_unix
		FROM omens
		ORDER BY received_at_unix DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recall recent Omens: %w", err)
	}
	defer rows.Close()

	var omens []OmenRecord
	for rows.Next() {
		var omen OmenRecord
		var gazeTurn int64
		var befallenAtUnix int64
		var receivedAtUnix int64

		if err := rows.Scan(
			&omen.OmenID,
			&omen.EyeID,
			&omen.GazeSigil,
			&gazeTurn,
			&befallenAtUnix,
			&receivedAtUnix,
		); err != nil {
			return nil, fmt.Errorf("read Omen: %w", err)
		}

		if gazeTurn < 1 {
			return nil, fmt.Errorf(
				"Omen %s has an invalid Gaze turn",
				omen.OmenID,
			)
		}

		omen.GazeTurn = uint64(gazeTurn)
		omen.BefallenAt = time.Unix(befallenAtUnix, 0).UTC()
		omen.ReceivedAt = time.Unix(receivedAtUnix, 0).UTC()
		omens = append(omens, omen)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Omens: %w", err)
	}

	return omens, nil
}

func (c *Chronicle) InscribeOracleSeal(
	sealHash string,
	createdAt time.Time,
	expiresAt time.Time,
) error {
	_, err := c.db.Exec(
		`INSERT INTO oracle_seals (
			seal_hash,
			created_at_unix,
			expires_at_unix
		) VALUES (?, ?, ?)`,
		sealHash,
		createdAt.Unix(),
		expiresAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("inscribe Oracle Seal: %w", err)
	}

	return nil
}

func (c *Chronicle) PairOracle(
	oracleID string,
	sealHash string,
	brandHash string,
	fcmToken string,
	pairedAt time.Time,
) error {
	oracleID = strings.TrimSpace(oracleID)
	sealHash = strings.TrimSpace(sealHash)
	brandHash = strings.TrimSpace(brandHash)
	fcmToken = strings.TrimSpace(fcmToken)

	if oracleID == "" {
		return fmt.Errorf("Oracle has no identity")
	}
	if sealHash == "" {
		return fmt.Errorf("Oracle pairing has no Seal")
	}
	if brandHash == "" {
		return fmt.Errorf("Oracle pairing has no Brand")
	}
	if fcmToken == "" {
		return fmt.Errorf("Oracle pairing has no FCM token")
	}

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin Oracle pairing: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := tx.Exec(
		`UPDATE oracle_seals
		 SET consumed_at_unix = ?, bound_oracle_id = ?
		 WHERE seal_hash = ?
		 	AND consumed_at_unix IS NULL
			AND expires_at_unix >= ?`,
		pairedAt.Unix(),
		oracleID,
		sealHash,
		pairedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("consume Oracle Seal: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read Oracle Seal receipt: %w", err)
	}

	if affected != 1 {
		return fmt.Errorf("Oracle Seal is invalid, expired or already consumed")
	}

	_, err = tx.Exec(
		`INSERT INTO oracles (
			oracle_id,
			brand_hash,
			paired_at_unix,
			revoked_at_unix
		) VALUES (?, ?, ?, NULL)
		ON CONFLICT(oracle_id) DO UPDATE SET
			brand_hash = excluded.brand_hash,
			paired_at_unix = excluded.paired_at_unix,
			revoked_at_unix = NULL`,
		oracleID,
		brandHash,
		pairedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("inscribe Oracle Brand: %w", err)
	}

	_, err = tx.Exec(
		`INSERT INTO oracle_tokens (
			oracle_id,
			fcm_token,
			refreshed_at_unix
		) VALUES (?, ?, ?)
		 ON CONFLICT(oracle_id) DO UPDATE SET
		 	fcm_token = excluded.fcm_token,
			refreshed_at_unix = excluded.refreshed_at_unix`,
		oracleID,
		fcmToken,
		pairedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("bind Oracle FCM Token: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Oracle pairing: %w", err)
	}

	return nil
}

func (c *Chronicle) RecallOracles() ([]OracleRecord, error) {
	rows, err := c.db.Query(
		`SELECT
			oracle_id,
			paired_at_unix,
			revoked_at_unix
		FROM oracles
		ORDER BY paired_at_unix DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("recall Oracles: %w", err)
	}
	defer rows.Close()

	var oracles []OracleRecord
	for rows.Next() {
		var oracle OracleRecord
		var pairedAtUnix int64
		var revokedAtUnix sql.NullInt64

		if err := rows.Scan(
			&oracle.OracleID,
			&pairedAtUnix,
			&revokedAtUnix,
		); err != nil {
			return nil, fmt.Errorf("read Oracle: %w", err)
		}

		oracle.PairedAt = time.Unix(pairedAtUnix, 0).UTC()
		if revokedAtUnix.Valid {
			revokedAt := time.Unix(revokedAtUnix.Int64, 0).UTC()
			oracle.RevokedAt = &revokedAt
		}

		oracles = append(oracles, oracle)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Oracles: %w", err)
	}

	return oracles, nil
}

func (c *Chronicle) RecallOracleBrandHash(
	oracleID string,
) (string, bool, error) {
	oracleID = strings.TrimSpace(oracleID)

	if oracleID == "" {
		return "", false, fmt.Errorf(
			"cannot recall Brand for an empty Oracle",
		)
	}

	var brandHash string

	err := c.db.QueryRow(
		`SELECT brand_hash
		 FROM oracles
		 WHERE oracle_id = ?
		 	AND revoked_at_unix IS NULL`,
		oracleID,
	).Scan(&brandHash)

	if err == sql.ErrNoRows {
		return "", false, nil
	}

	if err != nil {
		return "", false, fmt.Errorf(
			"recall Oracle Brand: %w",
			err,
		)
	}

	return brandHash, true, nil
}

func (c *Chronicle) RefreshOracleToken(
	oracleID string,
	fcmToken string,
	refreshedAt time.Time,
) error {
	oracleID = strings.TrimSpace(oracleID)
	fcmToken = strings.TrimSpace(fcmToken)

	if oracleID == "" {
		return fmt.Errorf("Oracle has no identity")
	}

	if fcmToken == "" {
		return fmt.Errorf("Oracle has no FCM Token")
	}

	result, err := c.db.Exec(
		`UPDATE oracle_tokens
		 	SET fcm_token = ?,
				refreshed_at_unix = ?
			WHERE oracle_id = ?
				AND EXISTS (
					SELECT 1
					FROM oracles
					WHERE oracle_id = ?
						AND revoked_at_unix IS NULL
				)`,
		fcmToken,
		refreshedAt.Unix(),
		oracleID,
		oracleID,
	)
	if err != nil {
		return fmt.Errorf("refresh Oracle FCM Token: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read Oracle token refresh: %w", err)
	}

	if affected != 1 {
		return fmt.Errorf("Oracle is unknown or revoked")
	}

	return nil
}

func (c *Chronicle) RecallOracleTokens() (
	[]string,
	error,
) {
	rows, err := c.db.Query(
		`SELECT DISTINCT oracle_tokens.fcm_token
		 FROM oracle_tokens
		 INNER JOIN oracles
			ON oracles.oracle_id = oracle_tokens.oracle_id
		 WHERE oracles.revoked_at_unix IS NULL
		 ORDER BY oracle_tokens.refreshed_at_unix DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("recall Oracle tokens: %w", err)
	}
	defer rows.Close()

	var tokens []string

	for rows.Next() {
		var token string

		if err := rows.Scan(&token); err != nil {
			return nil, fmt.Errorf("read Oracle token: %w", err)
		}

		token = strings.TrimSpace(token)
		if token != "" {
			tokens = append(tokens, token)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Oracle tokens: %w", err)
	}

	return tokens, nil
}
