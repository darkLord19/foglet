package cloud

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
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
	TeamID      string
	BotUserID   string
	BotToken    string
	InstalledAt time.Time
	UpdatedAt   time.Time
}

// PairingRequest is one pending one-time pairing code.
type PairingRequest struct {
	Code        string
	TeamID      string
	SlackUserID string
	ChannelID   string
	RootTS      string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	ClaimedAt   *time.Time
}

// PairingClaimResult is returned after successful pairing code claim.
type PairingClaimResult struct {
	TeamID      string
	SlackUserID string
	DeviceID    string
	DeviceToken string
}

const (
	jobStateQueued    = "queued"
	jobStateClaimed   = "claimed"
	jobStateCompleted = "completed"
	jobStateFailed    = "failed"

	jobKindStartSession = "start_session"
	jobKindFollowUp     = "follow_up"
)

// Job is a queued message routing unit for one paired device.
type Job struct {
	ID          string
	DeviceID    string
	TeamID      string
	ChannelID   string
	RootTS      string
	SlackUserID string
	Kind        string
	Repo        string
	Tool        string
	Model       string
	AutoPR      bool
	BranchName  string
	CommitMsg   string
	Prompt      string
	SessionID   string
	RunID       string
	Branch      string
	PRURL       string
	State       string
	Error       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ClaimedAt   *time.Time
	CompletedAt *time.Time
}

// JobCompletion stores completion payload submitted by device runtime.
type JobCompletion struct {
	JobID     string
	DeviceID  string
	Success   bool
	Error     string
	SessionID string
	RunID     string
	Branch    string
	PRURL     string
	CommitSHA string
	CommitMsg string
}

// NewStore opens or creates cloud sqlite state in dataDir.
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
		`CREATE TABLE IF NOT EXISTS devices (
			device_id TEXT PRIMARY KEY,
			token_hash TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS pairings (
			team_id TEXT NOT NULL,
			slack_user_id TEXT NOT NULL,
			device_id TEXT NOT NULL,
			paired_at TEXT NOT NULL,
			PRIMARY KEY(team_id, slack_user_id),
			FOREIGN KEY(device_id) REFERENCES devices(device_id)
		);`,
		`CREATE TABLE IF NOT EXISTS pairing_requests (
			code TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			slack_user_id TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			root_ts TEXT NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			claimed_at TEXT
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
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			device_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			root_ts TEXT NOT NULL,
			slack_user_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			repo TEXT,
			tool TEXT,
			model TEXT,
			autopr INTEGER NOT NULL DEFAULT 0,
			branch_name TEXT,
			commit_msg TEXT,
			prompt TEXT NOT NULL,
			session_id TEXT,
			run_id TEXT,
			branch TEXT,
			pr_url TEXT,
			state TEXT NOT NULL,
			error TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			claimed_at TEXT,
			completed_at TEXT,
			commit_sha TEXT,
			commit_msg_result TEXT,
			FOREIGN KEY(device_id) REFERENCES devices(device_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_pairing_requests_user ON pairing_requests(team_id, slack_user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_device_state_created ON jobs(device_id, state, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_team_channel_root ON jobs(team_id, channel_id, root_ts, created_at DESC);`,
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

// CreatePairingRequest creates a short-lived one-time pairing request.
func (s *Store) CreatePairingRequest(teamID, slackUserID, channelID, rootTS string, ttl time.Duration) (PairingRequest, error) {
	teamID = strings.TrimSpace(teamID)
	slackUserID = strings.TrimSpace(slackUserID)
	channelID = strings.TrimSpace(channelID)
	rootTS = strings.TrimSpace(rootTS)
	if teamID == "" || slackUserID == "" || channelID == "" || rootTS == "" {
		return PairingRequest{}, errors.New("team_id, slack_user_id, channel_id, and root_ts are required")
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}

	createdAt := time.Now().UTC()
	expiresAt := createdAt.Add(ttl)
	var code string
	for i := 0; i < 5; i++ {
		token, err := randomToken(4)
		if err != nil {
			return PairingRequest{}, err
		}
		code = strings.ToUpper(token)
		_, err = s.db.Exec(
			`INSERT INTO pairing_requests(code, team_id, slack_user_id, channel_id, root_ts, created_at, expires_at)
			 VALUES(?, ?, ?, ?, ?, ?, ?)`,
			code,
			teamID,
			slackUserID,
			channelID,
			rootTS,
			createdAt.Format(time.RFC3339Nano),
			expiresAt.Format(time.RFC3339Nano),
		)
		if err == nil {
			return PairingRequest{
				Code:        code,
				TeamID:      teamID,
				SlackUserID: slackUserID,
				ChannelID:   channelID,
				RootTS:      rootTS,
				CreatedAt:   createdAt,
				ExpiresAt:   expiresAt,
			}, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "constraint") {
			return PairingRequest{}, fmt.Errorf("create pairing request: %w", err)
		}
	}
	return PairingRequest{}, fmt.Errorf("failed to create unique pairing code")
}

// ClaimPairingRequest atomically consumes a pairing code and binds user to device.
func (s *Store) ClaimPairingRequest(code, deviceID, deviceToken string) (PairingClaimResult, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	deviceID = strings.TrimSpace(deviceID)
	deviceToken = strings.TrimSpace(deviceToken)
	if code == "" || deviceID == "" {
		return PairingClaimResult{}, errors.New("code and device_id are required")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return PairingClaimResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	var req PairingRequest
	var createdAtRaw string
	var expiresAtRaw string
	var claimedAtRaw sql.NullString
	err = tx.QueryRow(
		`SELECT code, team_id, slack_user_id, channel_id, root_ts, created_at, expires_at, claimed_at
		   FROM pairing_requests
		  WHERE code = ?`,
		code,
	).Scan(
		&req.Code,
		&req.TeamID,
		&req.SlackUserID,
		&req.ChannelID,
		&req.RootTS,
		&createdAtRaw,
		&expiresAtRaw,
		&claimedAtRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PairingClaimResult{}, errors.New("invalid pairing code")
	}
	if err != nil {
		return PairingClaimResult{}, fmt.Errorf("load pairing request: %w", err)
	}

	req.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtRaw)
	if err != nil {
		return PairingClaimResult{}, fmt.Errorf("parse pairing request created_at: %w", err)
	}
	req.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAtRaw)
	if err != nil {
		return PairingClaimResult{}, fmt.Errorf("parse pairing request expires_at: %w", err)
	}
	if claimedAtRaw.Valid {
		claimedAt, parseErr := time.Parse(time.RFC3339Nano, claimedAtRaw.String)
		if parseErr != nil {
			return PairingClaimResult{}, fmt.Errorf("parse pairing request claimed_at: %w", parseErr)
		}
		req.ClaimedAt = &claimedAt
	}
	if req.ClaimedAt != nil {
		return PairingClaimResult{}, errors.New("pairing code already claimed")
	}
	if !req.ExpiresAt.After(time.Now().UTC()) {
		return PairingClaimResult{}, errors.New("pairing code expired")
	}

	var existingDevice string
	err = tx.QueryRow(
		`SELECT device_id FROM pairings WHERE team_id = ? AND slack_user_id = ?`,
		req.TeamID,
		req.SlackUserID,
	).Scan(&existingDevice)
	if err == nil && existingDevice != deviceID {
		return PairingClaimResult{}, errors.New("user is already paired to another device; unpair first")
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return PairingClaimResult{}, fmt.Errorf("check existing pairing: %w", err)
	}

	issuedToken, err := s.ensureDeviceToken(tx, deviceID, deviceToken)
	if err != nil {
		return PairingClaimResult{}, err
	}

	if _, err := tx.Exec(
		`INSERT INTO pairings(team_id, slack_user_id, device_id, paired_at)
		 VALUES(?, ?, ?, ?)
		 ON CONFLICT(team_id, slack_user_id) DO UPDATE SET
		   device_id=excluded.device_id,
		   paired_at=excluded.paired_at`,
		req.TeamID,
		req.SlackUserID,
		deviceID,
		nowRFC3339Nano(),
	); err != nil {
		return PairingClaimResult{}, fmt.Errorf("save pairing: %w", err)
	}

	if _, err := tx.Exec(
		`UPDATE pairing_requests
		    SET claimed_at = ?
		  WHERE code = ?`,
		nowRFC3339Nano(),
		code,
	); err != nil {
		return PairingClaimResult{}, fmt.Errorf("mark pairing claimed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return PairingClaimResult{}, fmt.Errorf("commit tx: %w", err)
	}
	tx = nil

	return PairingClaimResult{
		TeamID:      req.TeamID,
		SlackUserID: req.SlackUserID,
		DeviceID:    deviceID,
		DeviceToken: issuedToken,
	}, nil
}

func (s *Store) ensureDeviceToken(tx *sql.Tx, deviceID, presentedToken string) (string, error) {
	var tokenHash string
	err := tx.QueryRow(
		`SELECT token_hash FROM devices WHERE device_id = ?`,
		deviceID,
	).Scan(&tokenHash)
	switch {
	case err == nil:
		if presentedToken == "" {
			return "", errors.New("device token is required for existing device")
		}
		if !constantTimeHashEqual(tokenHash, tokenHashHex(presentedToken)) {
			return "", errors.New("invalid device token")
		}
		return "", nil
	case errors.Is(err, sql.ErrNoRows):
	default:
		return "", fmt.Errorf("load device: %w", err)
	}

	issuedToken, err := randomToken(24)
	if err != nil {
		return "", fmt.Errorf("generate device token: %w", err)
	}
	now := nowRFC3339Nano()
	if _, err := tx.Exec(
		`INSERT INTO devices(device_id, token_hash, created_at, updated_at)
		 VALUES(?, ?, ?, ?)`,
		deviceID,
		tokenHashHex(issuedToken),
		now,
		now,
	); err != nil {
		return "", fmt.Errorf("insert device: %w", err)
	}
	return issuedToken, nil
}

// AuthenticateDevice validates device token for cloud agent APIs.
func (s *Store) AuthenticateDevice(deviceID, token string) error {
	deviceID = strings.TrimSpace(deviceID)
	token = strings.TrimSpace(token)
	if deviceID == "" || token == "" {
		return errors.New("device_id and token are required")
	}
	var storedHash string
	err := s.db.QueryRow(
		`SELECT token_hash FROM devices WHERE device_id = ?`,
		deviceID,
	).Scan(&storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("unknown device")
	}
	if err != nil {
		return fmt.Errorf("load device token: %w", err)
	}
	if !constantTimeHashEqual(storedHash, tokenHashHex(token)) {
		return errors.New("invalid device token")
	}
	return nil
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

	now := nowRFC3339Nano()
	if _, err := s.db.Exec(
		`INSERT INTO devices(device_id, token_hash, created_at, updated_at)
		 VALUES(?, ?, ?, ?)
		 ON CONFLICT(device_id) DO UPDATE SET updated_at=excluded.updated_at`,
		deviceID,
		"",
		now,
		now,
	); err != nil {
		return fmt.Errorf("upsert device: %w", err)
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

// UnpairStrict removes pairing only if it belongs to the given device.
func (s *Store) UnpairStrict(teamID, slackUserID, deviceID string) error {
	teamID = strings.TrimSpace(teamID)
	slackUserID = strings.TrimSpace(slackUserID)
	deviceID = strings.TrimSpace(deviceID)
	if teamID == "" || slackUserID == "" || deviceID == "" {
		return errors.New("team_id, slack_user_id, and device_id are required")
	}
	res, err := s.db.Exec(
		`DELETE FROM pairings
		  WHERE team_id = ? AND slack_user_id = ? AND device_id = ?`,
		teamID,
		slackUserID,
		deviceID,
	)
	if err != nil {
		return fmt.Errorf("unpair strict: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return errors.New("pairing not found for device")
	}
	return nil
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

// EnqueueJob inserts one new queued job for a paired device.
func (s *Store) EnqueueJob(job Job) (Job, error) {
	job.DeviceID = strings.TrimSpace(job.DeviceID)
	job.TeamID = strings.TrimSpace(job.TeamID)
	job.ChannelID = strings.TrimSpace(job.ChannelID)
	job.RootTS = strings.TrimSpace(job.RootTS)
	job.SlackUserID = strings.TrimSpace(job.SlackUserID)
	job.Kind = strings.TrimSpace(job.Kind)
	job.Repo = strings.TrimSpace(job.Repo)
	job.Tool = strings.TrimSpace(job.Tool)
	job.Model = strings.TrimSpace(job.Model)
	job.BranchName = strings.TrimSpace(job.BranchName)
	job.CommitMsg = strings.TrimSpace(job.CommitMsg)
	job.Prompt = strings.TrimSpace(job.Prompt)
	job.SessionID = strings.TrimSpace(job.SessionID)
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if job.State == "" {
		job.State = jobStateQueued
	}
	if job.DeviceID == "" || job.TeamID == "" || job.ChannelID == "" || job.RootTS == "" || job.SlackUserID == "" || job.Kind == "" || job.Prompt == "" {
		return Job{}, errors.New("job missing required fields")
	}
	switch job.Kind {
	case jobKindStartSession:
		if job.Repo == "" {
			return Job{}, errors.New("start_session requires repo")
		}
	case jobKindFollowUp:
		if job.SessionID == "" {
			return Job{}, errors.New("follow_up requires session_id")
		}
	default:
		return Job{}, fmt.Errorf("unknown job kind %q", job.Kind)
	}
	if job.State != jobStateQueued {
		return Job{}, errors.New("new jobs must be queued")
	}

	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	_, err := s.db.Exec(
		`INSERT INTO jobs(
			id, device_id, team_id, channel_id, root_ts, slack_user_id, kind, repo, tool, model, autopr,
			branch_name, commit_msg, prompt, session_id, run_id, branch, pr_url, state, error,
			created_at, updated_at, claimed_at, completed_at, commit_sha, commit_msg_result
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, '', '')`,
		job.ID,
		job.DeviceID,
		job.TeamID,
		job.ChannelID,
		job.RootTS,
		job.SlackUserID,
		job.Kind,
		job.Repo,
		job.Tool,
		job.Model,
		boolToInt(job.AutoPR),
		job.BranchName,
		job.CommitMsg,
		job.Prompt,
		job.SessionID,
		"",
		"",
		"",
		job.State,
		"",
		job.CreatedAt.Format(time.RFC3339Nano),
		job.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Job{}, fmt.Errorf("enqueue job: %w", err)
	}
	return job, nil
}

// ClaimNextJob atomically transitions the oldest queued job for device to claimed.
func (s *Store) ClaimNextJob(deviceID string) (Job, bool, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return Job{}, false, errors.New("device_id is required")
	}
	for i := 0; i < 4; i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return Job{}, false, fmt.Errorf("begin tx: %w", err)
		}

		var jobID string
		err = tx.QueryRow(
			`SELECT id FROM jobs
			  WHERE device_id = ? AND state = ?
			  ORDER BY created_at ASC
			  LIMIT 1`,
			deviceID,
			jobStateQueued,
		).Scan(&jobID)
		if errors.Is(err, sql.ErrNoRows) {
			_ = tx.Rollback()
			return Job{}, false, nil
		}
		if err != nil {
			_ = tx.Rollback()
			return Job{}, false, fmt.Errorf("claim select: %w", err)
		}

		res, err := tx.Exec(
			`UPDATE jobs
			    SET state = ?, claimed_at = ?, updated_at = ?
			  WHERE id = ? AND state = ?`,
			jobStateClaimed,
			nowRFC3339Nano(),
			nowRFC3339Nano(),
			jobID,
			jobStateQueued,
		)
		if err != nil {
			_ = tx.Rollback()
			return Job{}, false, fmt.Errorf("claim update: %w", err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			_ = tx.Rollback()
			return Job{}, false, fmt.Errorf("rows affected: %w", err)
		}
		if rows == 0 {
			_ = tx.Rollback()
			continue
		}

		job, found, err := getJobTx(tx, jobID)
		if err != nil {
			_ = tx.Rollback()
			return Job{}, false, err
		}
		if !found {
			_ = tx.Rollback()
			return Job{}, false, fmt.Errorf("claimed job disappeared")
		}
		if err := tx.Commit(); err != nil {
			return Job{}, false, fmt.Errorf("commit tx: %w", err)
		}
		return job, true, nil
	}
	return Job{}, false, nil
}

// CompleteJob stores terminal status and execution outputs.
func (s *Store) CompleteJob(in JobCompletion) (Job, error) {
	in.JobID = strings.TrimSpace(in.JobID)
	in.DeviceID = strings.TrimSpace(in.DeviceID)
	in.Error = strings.TrimSpace(in.Error)
	in.SessionID = strings.TrimSpace(in.SessionID)
	in.RunID = strings.TrimSpace(in.RunID)
	in.Branch = strings.TrimSpace(in.Branch)
	in.PRURL = strings.TrimSpace(in.PRURL)
	in.CommitSHA = strings.TrimSpace(in.CommitSHA)
	in.CommitMsg = strings.TrimSpace(in.CommitMsg)
	if in.JobID == "" || in.DeviceID == "" {
		return Job{}, errors.New("job_id and device_id are required")
	}

	nextState := jobStateCompleted
	if !in.Success {
		nextState = jobStateFailed
	}
	if !in.Success && in.Error == "" {
		in.Error = "job failed"
	}

	res, err := s.db.Exec(
		`UPDATE jobs
		    SET state = ?,
		        error = ?,
		        session_id = CASE WHEN ? = '' THEN session_id ELSE ? END,
		        run_id = CASE WHEN ? = '' THEN run_id ELSE ? END,
		        branch = CASE WHEN ? = '' THEN branch ELSE ? END,
		        pr_url = CASE WHEN ? = '' THEN pr_url ELSE ? END,
		        commit_sha = CASE WHEN ? = '' THEN commit_sha ELSE ? END,
		        commit_msg_result = CASE WHEN ? = '' THEN commit_msg_result ELSE ? END,
		        completed_at = ?,
		        updated_at = ?
		  WHERE id = ? AND device_id = ?`,
		nextState,
		in.Error,
		in.SessionID, in.SessionID,
		in.RunID, in.RunID,
		in.Branch, in.Branch,
		in.PRURL, in.PRURL,
		in.CommitSHA, in.CommitSHA,
		in.CommitMsg, in.CommitMsg,
		nowRFC3339Nano(),
		nowRFC3339Nano(),
		in.JobID,
		in.DeviceID,
	)
	if err != nil {
		return Job{}, fmt.Errorf("complete job: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return Job{}, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return Job{}, errors.New("job not found for device")
	}

	job, found, err := s.GetJob(in.JobID)
	if err != nil {
		return Job{}, err
	}
	if !found {
		return Job{}, errors.New("job not found after update")
	}
	return job, nil
}

// GetJob returns one job by id.
func (s *Store) GetJob(jobID string) (Job, bool, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return Job{}, false, errors.New("job_id is required")
	}
	return getJobTx(s.db, jobID)
}

type queryRower interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func getJobTx(q queryRower, jobID string) (Job, bool, error) {
	var job Job
	var autopr int
	var createdAtRaw string
	var updatedAtRaw string
	var claimedAtRaw sql.NullString
	var completedAtRaw sql.NullString
	err := q.QueryRow(
		`SELECT id, device_id, team_id, channel_id, root_ts, slack_user_id, kind, repo, tool, model, autopr,
		        branch_name, commit_msg, prompt, session_id, run_id, branch, pr_url, state, error,
		        created_at, updated_at, claimed_at, completed_at
		   FROM jobs WHERE id = ?`,
		jobID,
	).Scan(
		&job.ID,
		&job.DeviceID,
		&job.TeamID,
		&job.ChannelID,
		&job.RootTS,
		&job.SlackUserID,
		&job.Kind,
		&job.Repo,
		&job.Tool,
		&job.Model,
		&autopr,
		&job.BranchName,
		&job.CommitMsg,
		&job.Prompt,
		&job.SessionID,
		&job.RunID,
		&job.Branch,
		&job.PRURL,
		&job.State,
		&job.Error,
		&createdAtRaw,
		&updatedAtRaw,
		&claimedAtRaw,
		&completedAtRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("get job: %w", err)
	}
	job.AutoPR = autopr == 1
	var parseErr error
	job.CreatedAt, parseErr = time.Parse(time.RFC3339Nano, createdAtRaw)
	if parseErr != nil {
		return Job{}, false, fmt.Errorf("parse created_at: %w", parseErr)
	}
	job.UpdatedAt, parseErr = time.Parse(time.RFC3339Nano, updatedAtRaw)
	if parseErr != nil {
		return Job{}, false, fmt.Errorf("parse updated_at: %w", parseErr)
	}
	if claimedAtRaw.Valid {
		claimedAt, err := time.Parse(time.RFC3339Nano, claimedAtRaw.String)
		if err != nil {
			return Job{}, false, fmt.Errorf("parse claimed_at: %w", err)
		}
		job.ClaimedAt = &claimedAt
	}
	if completedAtRaw.Valid {
		completedAt, err := time.Parse(time.RFC3339Nano, completedAtRaw.String)
		if err != nil {
			return Job{}, false, fmt.Errorf("parse completed_at: %w", err)
		}
		job.CompletedAt = &completedAt
	}
	return job, true, nil
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

func tokenHashHex(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func constantTimeHashEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nowRFC3339Nano() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
