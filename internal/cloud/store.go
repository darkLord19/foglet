package cloud

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultDBName  = "fogcloud.db"
	defaultKeyName = "cloud.key"
)

// Store persists multi-tenant Slack routing metadata.
type Store struct {
	db  *sql.DB
	key []byte
}

// Installation stores one Slack workspace installation.
type Installation struct {
	TeamID     string
	BotUserID  string
	BotToken   string
	InstalledAt time.Time
	UpdatedAt  time.Time
}

// NewStore opens or creates the cloud sqlite store.
func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	keyPath := filepath.Join(dataDir, defaultKeyName)
	key, err := loadOrCreateMasterKey(keyPath)
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, defaultDBName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	s := &Store{db: db, key: key}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	stmts := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA busy_timeout = 5000;`,
		`CREATE TABLE IF NOT EXISTS installations (
			team_id TEXT PRIMARY KEY,
			bot_user_id TEXT,
			bot_token_cipher BLOB NOT NULL,
			installed_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS pairings (
			team_id TEXT NOT NULL,
			slack_user_id TEXT NOT NULL,
			device_id TEXT NOT NULL,
			paired_at TEXT NOT NULL,
			PRIMARY KEY(team_id, slack_user_id)
		);`,
		`CREATE TABLE IF NOT EXISTS thread_sessions (
			team_id TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			root_ts TEXT NOT NULL,
			session_id TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(team_id, channel_id, root_ts)
		);`,
		`CREATE TABLE IF NOT EXISTS seen_events (
			team_id TEXT NOT NULL,
			event_id TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY(team_id, event_id)
		);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// SaveInstallation upserts encrypted bot credentials per workspace.
func (s *Store) SaveInstallation(teamID, botUserID, botToken string) error {
	teamID = strings.TrimSpace(teamID)
	botUserID = strings.TrimSpace(botUserID)
	botToken = strings.TrimSpace(botToken)
	if teamID == "" {
		return errors.New("team_id is required")
	}
	if botToken == "" {
		return errors.New("bot token is required")
	}

	cipher, err := encrypt("slack_bot_token:"+teamID, []byte(botToken), s.key)
	if err != nil {
		return fmt.Errorf("encrypt bot token: %w", err)
	}
	now := nowRFC3339Nano()
	_, err = s.db.Exec(
		`INSERT INTO installations(team_id, bot_user_id, bot_token_cipher, installed_at, updated_at)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(team_id) DO UPDATE SET
		   bot_user_id=excluded.bot_user_id,
		   bot_token_cipher=excluded.bot_token_cipher,
		   updated_at=excluded.updated_at`,
		teamID,
		botUserID,
		cipher,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("save installation: %w", err)
	}
	return nil
}

// GetInstallation returns one workspace installation.
func (s *Store) GetInstallation(teamID string) (Installation, bool, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return Installation{}, false, errors.New("team_id is required")
	}

	var inst Installation
	var cipher []byte
	var installedAtRaw string
	var updatedAtRaw string
	err := s.db.QueryRow(
		`SELECT team_id, bot_user_id, bot_token_cipher, installed_at, updated_at
		   FROM installations
		  WHERE team_id = ?`,
		teamID,
	).Scan(&inst.TeamID, &inst.BotUserID, &cipher, &installedAtRaw, &updatedAtRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return Installation{}, false, nil
	}
	if err != nil {
		return Installation{}, false, fmt.Errorf("get installation: %w", err)
	}

	plain, err := decrypt("slack_bot_token:"+teamID, cipher, s.key)
	if err != nil {
		return Installation{}, false, err
	}
	inst.BotToken = string(plain)
	if inst.InstalledAt, err = time.Parse(time.RFC3339Nano, installedAtRaw); err != nil {
		return Installation{}, false, fmt.Errorf("parse installed_at: %w", err)
	}
	if inst.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtRaw); err != nil {
		return Installation{}, false, fmt.Errorf("parse updated_at: %w", err)
	}

	return inst, true, nil
}

// PairDevice enforces one device per (team,user) until explicit unpair.
func (s *Store) PairDevice(teamID, slackUserID, deviceID string) error {
	teamID = strings.TrimSpace(teamID)
	slackUserID = strings.TrimSpace(slackUserID)
	deviceID = strings.TrimSpace(deviceID)
	if teamID == "" || slackUserID == "" || deviceID == "" {
		return errors.New("team_id, slack_user_id, and device_id are required")
	}

	var existing string
	err := s.db.QueryRow(
		`SELECT device_id FROM pairings WHERE team_id = ? AND slack_user_id = ?`,
		teamID, slackUserID,
	).Scan(&existing)
	switch {
	case err == nil:
		if existing != deviceID {
			return fmt.Errorf("user is already paired to another device; unpair first")
		}
		return nil
	case errors.Is(err, sql.ErrNoRows):
	default:
		return fmt.Errorf("check existing pairing: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO pairings(team_id, slack_user_id, device_id, paired_at)
		 VALUES(?, ?, ?, ?)`,
		teamID, slackUserID, deviceID, nowRFC3339Nano(),
	)
	if err != nil {
		return fmt.Errorf("pair device: %w", err)
	}
	return nil
}

func (s *Store) UnpairDevice(teamID, slackUserID string) error {
	teamID = strings.TrimSpace(teamID)
	slackUserID = strings.TrimSpace(slackUserID)
	if teamID == "" || slackUserID == "" {
		return errors.New("team_id and slack_user_id are required")
	}
	_, err := s.db.Exec(
		`DELETE FROM pairings WHERE team_id = ? AND slack_user_id = ?`,
		teamID, slackUserID,
	)
	if err != nil {
		return fmt.Errorf("unpair device: %w", err)
	}
	return nil
}

func (s *Store) GetPairing(teamID, slackUserID string) (string, bool, error) {
	teamID = strings.TrimSpace(teamID)
	slackUserID = strings.TrimSpace(slackUserID)
	if teamID == "" || slackUserID == "" {
		return "", false, errors.New("team_id and slack_user_id are required")
	}

	var deviceID string
	err := s.db.QueryRow(
		`SELECT device_id FROM pairings WHERE team_id = ? AND slack_user_id = ?`,
		teamID, slackUserID,
	).Scan(&deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get pairing: %w", err)
	}
	return deviceID, true, nil
}

func (s *Store) UpsertThreadSession(teamID, channelID, rootTS, sessionID string) error {
	teamID = strings.TrimSpace(teamID)
	channelID = strings.TrimSpace(channelID)
	rootTS = strings.TrimSpace(rootTS)
	sessionID = strings.TrimSpace(sessionID)
	if teamID == "" || channelID == "" || rootTS == "" || sessionID == "" {
		return errors.New("team_id, channel_id, root_ts, and session_id are required")
	}

	_, err := s.db.Exec(
		`INSERT INTO thread_sessions(team_id, channel_id, root_ts, session_id, updated_at)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(team_id, channel_id, root_ts) DO UPDATE SET
		   session_id=excluded.session_id,
		   updated_at=excluded.updated_at`,
		teamID, channelID, rootTS, sessionID, nowRFC3339Nano(),
	)
	if err != nil {
		return fmt.Errorf("upsert thread session: %w", err)
	}
	return nil
}

func (s *Store) GetThreadSession(teamID, channelID, rootTS string) (string, bool, error) {
	teamID = strings.TrimSpace(teamID)
	channelID = strings.TrimSpace(channelID)
	rootTS = strings.TrimSpace(rootTS)
	if teamID == "" || channelID == "" || rootTS == "" {
		return "", false, errors.New("team_id, channel_id, and root_ts are required")
	}
	var sessionID string
	err := s.db.QueryRow(
		`SELECT session_id FROM thread_sessions
		  WHERE team_id = ? AND channel_id = ? AND root_ts = ?`,
		teamID, channelID, rootTS,
	).Scan(&sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get thread session: %w", err)
	}
	return sessionID, true, nil
}

// RecordEventID returns true when this event id was new and recorded.
func (s *Store) RecordEventID(teamID, eventID string) (bool, error) {
	teamID = strings.TrimSpace(teamID)
	eventID = strings.TrimSpace(eventID)
	if teamID == "" || eventID == "" {
		return false, errors.New("team_id and event_id are required")
	}

	res, err := s.db.Exec(
		`INSERT INTO seen_events(team_id, event_id, created_at)
		 VALUES(?, ?, ?)`,
		teamID, eventID, nowRFC3339Nano(),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "constraint") {
			return false, nil
		}
		return false, fmt.Errorf("record event id: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return rows > 0, nil
}

func nowRFC3339Nano() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
