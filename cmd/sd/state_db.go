package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	stateDBFileName      = "state.db"
	stateSchemaVersion   = 1
	configPrimaryKey     = "default"
	metaSchemaVersionKey = "schema_version"
	metaLegacyMigrated   = "legacy_migrated_v1"
	metaCheckpointsMigrated = "legacy_checkpoints_migrated_v1"
)

func stateDBPath(stateDir string) string {
	return filepath.Join(stateDir, stateDBFileName)
}

func resolveStateDirForPath(path string) string {
	if stateDir := findStateDirFromPath(path); stateDir != "" {
		return stateDir
	}
	dir := filepath.Clean(filepath.Dir(path))
	if filepath.Base(dir) == stateDirName {
		return dir
	}
	return dir
}

func openStateDB(stateDir string) (*sql.DB, error) {
	if strings.TrimSpace(stateDir) == "" {
		return nil, errors.New("empty state dir")
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", stateDBPath(stateDir))
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := initStateDBSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`INSERT INTO state_meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, metaSchemaVersionKey, fmt.Sprintf("%d", stateSchemaVersion)); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func initStateDBSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS state_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS state_config (
			config_key TEXT PRIMARY KEY,
			config_json TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS interactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			schema_version INTEGER NOT NULL,
			event_type TEXT,
			session_id TEXT,
			timestamp TEXT NOT NULL,
			command TEXT NOT NULL,
			args_json TEXT NOT NULL,
			exit_code INTEGER NOT NULL,
			in_progress INTEGER NOT NULL,
			input_preview TEXT,
			conversation_log TEXT,
			stdin_log TEXT,
			stdout_log TEXT,
			modified_files_json TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS hidden_sessions (
			session_key TEXT PRIMARY KEY
		)`,
		`CREATE TABLE IF NOT EXISTS session_numbers (
			session_key TEXT PRIMARY KEY,
			number INTEGER NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS conversation_messages (
			conversation_log TEXT NOT NULL,
			seq INTEGER NOT NULL,
			dt TEXT NOT NULL,
			role TEXT NOT NULL,
			text TEXT NOT NULL,
			PRIMARY KEY(conversation_log, seq)
		)`,
		`CREATE TABLE IF NOT EXISTS generated_specs (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			content TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS compact_bundle_meta (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			schema_version INTEGER NOT NULL,
			generated_at TEXT NOT NULL,
			max_bytes INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS compact_bundle_files (
			path TEXT PRIMARY KEY,
			size_bytes INTEGER NOT NULL,
			sha256 TEXT NOT NULL,
			content_b64 TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS checkpoints_state (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			payload_json TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func readConfigFromDB(stateDir string) (config, bool, error) {
	db, err := openStateDB(stateDir)
	if err != nil {
		return config{}, false, err
	}
	defer db.Close()

	var raw string
	err = db.QueryRow(`SELECT config_json FROM state_config WHERE config_key = ?`, configPrimaryKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return config{}, false, nil
	}
	if err != nil {
		return config{}, false, err
	}
	var cfg config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return config{}, false, err
	}
	return cfg, true, nil
}

func writeConfigToDB(stateDir string, cfg config) error {
	db, err := openStateDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO state_config(config_key, config_json) VALUES(?, ?)
		ON CONFLICT(config_key) DO UPDATE SET config_json=excluded.config_json`, configPrimaryKey, string(raw))
	return err
}

func readInteractionsFromDB(stateDir string) ([]interaction, error) {
	db, err := openStateDB(stateDir)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT schema_version, event_type, session_id, timestamp, command, args_json, exit_code, in_progress, input_preview, conversation_log, stdin_log, stdout_log, modified_files_json
		FROM interactions ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []interaction
	for rows.Next() {
		var (
			argsJSON          string
			modifiedFilesJSON string
			item              interaction
			inProgressInt     int
		)
		if err := rows.Scan(
			&item.SchemaVersion,
			&item.EventType,
			&item.SessionID,
			&item.Timestamp,
			&item.Command,
			&argsJSON,
			&item.ExitCode,
			&inProgressInt,
			&item.InputPreview,
			&item.ConversationLog,
			&item.StdinLog,
			&item.StdoutLog,
			&modifiedFilesJSON,
		); err != nil {
			return nil, err
		}
		item.InProgress = inProgressInt != 0
		if err := json.Unmarshal([]byte(nonEmpty(argsJSON, "[]")), &item.Args); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(nonEmpty(modifiedFilesJSON, "[]")), &item.ModifiedFiles); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func appendInteractionToDB(stateDir string, item interaction) error {
	db, err := openStateDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()

	argsJSON, err := json.Marshal(item.Args)
	if err != nil {
		return err
	}
	modifiedJSON, err := json.Marshal(item.ModifiedFiles)
	if err != nil {
		return err
	}
	inProgress := 0
	if item.InProgress {
		inProgress = 1
	}
	if item.SchemaVersion == 0 {
		item.SchemaVersion = interactionSchemaVersion
	}
	if strings.TrimSpace(item.Timestamp) == "" {
		item.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	_, err = db.Exec(`INSERT INTO interactions(schema_version, event_type, session_id, timestamp, command, args_json, exit_code, in_progress, input_preview, conversation_log, stdin_log, stdout_log, modified_files_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.SchemaVersion,
		item.EventType,
		item.SessionID,
		item.Timestamp,
		item.Command,
		string(argsJSON),
		item.ExitCode,
		inProgress,
		item.InputPreview,
		item.ConversationLog,
		item.StdinLog,
		item.StdoutLog,
		string(modifiedJSON),
	)
	return err
}

func replaceInteractionsInDB(stateDir string, interactions []interaction) error {
	db, err := openStateDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM interactions`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO interactions(schema_version, event_type, session_id, timestamp, command, args_json, exit_code, in_progress, input_preview, conversation_log, stdin_log, stdout_log, modified_files_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, item := range interactions {
		argsJSON, err := json.Marshal(item.Args)
		if err != nil {
			return err
		}
		modifiedJSON, err := json.Marshal(item.ModifiedFiles)
		if err != nil {
			return err
		}
		inProgress := 0
		if item.InProgress {
			inProgress = 1
		}
		schemaVersion := item.SchemaVersion
		if schemaVersion == 0 {
			schemaVersion = interactionSchemaVersion
		}
		if _, err := stmt.Exec(
			schemaVersion,
			item.EventType,
			item.SessionID,
			item.Timestamp,
			item.Command,
			string(argsJSON),
			item.ExitCode,
			inProgress,
			item.InputPreview,
			item.ConversationLog,
			item.StdinLog,
			item.StdoutLog,
			string(modifiedJSON),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func readHiddenSessionsFromDB(stateDir string) (map[string]struct{}, error) {
	db, err := openStateDB(stateDir)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT session_key FROM hidden_sessions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]struct{}{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = struct{}{}
	}
	return out, rows.Err()
}

func writeHiddenSessionsToDB(stateDir string, hidden map[string]struct{}) error {
	db, err := openStateDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM hidden_sessions`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO hidden_sessions(session_key) VALUES (?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	ids := make([]string, 0, len(hidden))
	for sessionKey := range hidden {
		sessionKey = strings.TrimSpace(sessionKey)
		if sessionKey != "" {
			ids = append(ids, sessionKey)
		}
	}
	sort.Strings(ids)
	for _, sessionKey := range ids {
		if _, err := stmt.Exec(sessionKey); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func readSessionIndexFromDB(stateDir string) (sessionIndex, error) {
	db, err := openStateDB(stateDir)
	if err != nil {
		return sessionIndex{}, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT session_key, number FROM session_numbers ORDER BY number ASC`)
	if err != nil {
		return sessionIndex{}, err
	}
	defer rows.Close()

	idx := sessionIndex{Entries: map[string]int{}}
	max := -1
	for rows.Next() {
		var key string
		var n int
		if err := rows.Scan(&key, &n); err != nil {
			return sessionIndex{}, err
		}
		idx.Entries[key] = n
		if n > max {
			max = n
		}
	}
	if err := rows.Err(); err != nil {
		return sessionIndex{}, err
	}
	idx.NextID = max + 1
	if idx.NextID < 0 {
		idx.NextID = 0
	}
	return idx, nil
}

func writeSessionIndexToDB(stateDir string, idx sessionIndex) error {
	db, err := openStateDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()

	if idx.Entries == nil {
		idx.Entries = map[string]int{}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM session_numbers`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO session_numbers(session_key, number) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	keys := make([]string, 0, len(idx.Entries))
	for key := range idx.Entries {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return idx.Entries[keys[i]] < idx.Entries[keys[j]]
	})
	for _, key := range keys {
		if _, err := stmt.Exec(key, idx.Entries[key]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func conversationLogKeyFromPath(path string) (stateDir string, key string, err error) {
	stateDir = resolveStateDirForPath(path)
	if strings.TrimSpace(stateDir) == "" {
		return "", "", fmt.Errorf("cannot resolve state dir from %q", path)
	}
	cleanPath := filepath.Clean(path)
	repoRoot := filepath.Dir(stateDir)
	if filepath.Base(stateDir) != stateDirName {
		repoRoot = stateDir
	}
	if rel, relErr := filepath.Rel(repoRoot, cleanPath); relErr == nil && !strings.HasPrefix(rel, "..") {
		return stateDir, filepath.ToSlash(rel), nil
	}
	return stateDir, filepath.ToSlash(cleanPath), nil
}

func readConversationLogFromDB(stateDir, conversationKey string) ([]conversationMessage, error) {
	db, err := openStateDB(stateDir)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT dt, role, text FROM conversation_messages WHERE conversation_log = ? ORDER BY seq ASC`, conversationKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []conversationMessage{}
	for rows.Next() {
		var msg conversationMessage
		if err := rows.Scan(&msg.Dt, &msg.Role, &msg.Text); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, rows.Err()
}

func writeConversationLogToDB(stateDir, conversationKey string, messages []conversationMessage) error {
	db, err := openStateDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM conversation_messages WHERE conversation_log = ?`, conversationKey); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO conversation_messages(conversation_log, seq, dt, role, text) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i := range messages {
		if strings.TrimSpace(messages[i].Dt) == "" {
			messages[i].Dt = time.Now().UTC().Format(time.RFC3339)
		}
		if _, err := stmt.Exec(conversationKey, i, messages[i].Dt, messages[i].Role, messages[i].Text); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func upsertGeneratedSpecInDB(stateDir, content string) error {
	db, err := openStateDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO generated_specs(id, content, updated_at) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`,
		content, time.Now().UTC().Format(time.RFC3339))
	return err
}

func readGeneratedSpecFromDB(stateDir string) (string, error) {
	db, err := openStateDB(stateDir)
	if err != nil {
		return "", err
	}
	defer db.Close()
	var content string
	if err := db.QueryRow(`SELECT content FROM generated_specs WHERE id = 1`).Scan(&content); err != nil {
		return "", err
	}
	return content, nil
}

func writeCompactBundleToDB(stateDir string, payload compactedSessions) error {
	db, err := openStateDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM compact_bundle_meta`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM compact_bundle_files`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO compact_bundle_meta(id, schema_version, generated_at, max_bytes) VALUES (1, ?, ?, ?)`,
		payload.SchemaVersion, payload.GeneratedAt, payload.MaxBytes); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO compact_bundle_files(path, size_bytes, sha256, content_b64) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, file := range payload.Files {
		if _, err := stmt.Exec(file.Path, file.SizeBytes, file.SHA256, file.ContentB64); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func readCompactBundleFromDB(stateDir string) (compactedSessions, error) {
	db, err := openStateDB(stateDir)
	if err != nil {
		return compactedSessions{}, err
	}
	defer db.Close()
	var out compactedSessions
	if err := db.QueryRow(`SELECT schema_version, generated_at, max_bytes FROM compact_bundle_meta WHERE id = 1`).
		Scan(&out.SchemaVersion, &out.GeneratedAt, &out.MaxBytes); err != nil {
		return compactedSessions{}, err
	}
	rows, err := db.Query(`SELECT path, size_bytes, sha256, content_b64 FROM compact_bundle_files ORDER BY path ASC`)
	if err != nil {
		return compactedSessions{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var f compactedSessionFile
		if err := rows.Scan(&f.Path, &f.SizeBytes, &f.SHA256, &f.ContentB64); err != nil {
			return compactedSessions{}, err
		}
		out.Files = append(out.Files, f)
	}
	return out, rows.Err()
}

func upsertCheckpointsStateRawInDB(stateDir, raw string) error {
	db, err := openStateDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = `{"version":1,"next_id":1,"entries":[]}`
	}
	var validated any
	if err := json.Unmarshal([]byte(raw), &validated); err != nil {
		return err
	}
	normalized, err := json.Marshal(validated)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO checkpoints_state(id, payload_json, updated_at) VALUES(1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET payload_json=excluded.payload_json, updated_at=excluded.updated_at`,
		string(normalized),
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func migrateLegacyStateFiles(repoRoot, stateDir string) error {
	db, err := openStateDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()

	var legacyMigrated bool
	var migrated string
	if err := db.QueryRow(`SELECT value FROM state_meta WHERE key = ?`, metaLegacyMigrated).Scan(&migrated); err == nil && migrated == "1" {
		legacyMigrated = true
	}

	if !legacyMigrated {
		// Config
		var cfgCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM state_config`).Scan(&cfgCount); err != nil {
			return err
		}
		if cfgCount == 0 {
			cfgPath := filepath.Join(stateDir, "config.json")
			if raw, err := os.ReadFile(cfgPath); err == nil {
				var cfg config
				if json.Unmarshal(raw, &cfg) == nil {
					if err := writeConfigToDB(stateDir, cfg); err != nil {
						return err
					}
				}
			}
		}

		// Interactions
		var interactionCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM interactions`).Scan(&interactionCount); err != nil {
			return err
		}
		if interactionCount == 0 {
			legacyPath := filepath.Join(stateDir, "interactions.ndjson")
			if items, err := readInteractionTimelineFile(legacyPath); err == nil && len(items) > 0 {
				if err := replaceInteractionsInDB(stateDir, items); err != nil {
					return err
				}
			}
		}

		// Hidden sessions
		var hiddenCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM hidden_sessions`).Scan(&hiddenCount); err != nil {
			return err
		}
		if hiddenCount == 0 {
			path := filepath.Join(stateDir, "sessions.hidden.json")
			if raw, err := os.ReadFile(path); err == nil {
				var payload hiddenSessions
				if json.Unmarshal(raw, &payload) == nil {
					set := map[string]struct{}{}
					for _, id := range payload.HiddenSessionIDs {
						id = strings.TrimSpace(id)
						if id != "" {
							set[id] = struct{}{}
						}
					}
					if err := writeHiddenSessionsToDB(stateDir, set); err != nil {
						return err
					}
				}
			}
		}

		// Session numbers
		var sessionNumberCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM session_numbers`).Scan(&sessionNumberCount); err != nil {
			return err
		}
		if sessionNumberCount == 0 {
			path := filepath.Join(stateDir, "session_index.json")
			if raw, err := os.ReadFile(path); err == nil {
				var idx sessionIndex
				if json.Unmarshal(raw, &idx) == nil {
					if idx.Entries == nil {
						idx.Entries = map[string]int{}
					}
					if err := writeSessionIndexToDB(stateDir, idx); err != nil {
						return err
					}
				}
			}
		}

		// Conversation logs from referenced interactions + sessions directory.
		var convoCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM conversation_messages`).Scan(&convoCount); err != nil {
			return err
		}
		if convoCount == 0 {
			migratedLogs := map[string]struct{}{}
			interactions, err := readInteractionsFromDB(stateDir)
			if err == nil {
				for _, item := range interactions {
					rel := strings.TrimSpace(item.ConversationLog)
					if rel == "" {
						continue
					}
					if _, exists := migratedLogs[rel]; exists {
						continue
					}
					abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
					msgs, readErr := readConversationLogFile(abs)
					if readErr != nil {
						continue
					}
					if writeErr := writeConversationLogToDB(stateDir, rel, msgs); writeErr != nil {
						return writeErr
					}
					migratedLogs[rel] = struct{}{}
				}
			}
			sessionsDir := filepath.Join(stateDir, "sessions")
			if entries, err := os.ReadDir(sessionsDir); err == nil {
				for _, entry := range entries {
					if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conversation.json") {
						continue
					}
					rel := filepath.ToSlash(filepath.Join(stateDirName, "sessions", entry.Name()))
					if _, exists := migratedLogs[rel]; exists {
						continue
					}
					abs := filepath.Join(sessionsDir, entry.Name())
					msgs, readErr := readConversationLogFile(abs)
					if readErr != nil {
						continue
					}
					if writeErr := writeConversationLogToDB(stateDir, rel, msgs); writeErr != nil {
						return writeErr
					}
					migratedLogs[rel] = struct{}{}
				}
			}
		}

		// Generated spec
		var hasGenerated int
		if err := db.QueryRow(`SELECT COUNT(*) FROM generated_specs`).Scan(&hasGenerated); err != nil {
			return err
		}
		if hasGenerated == 0 {
			path := filepath.Join(stateDir, "spec.generated.md")
			if raw, err := os.ReadFile(path); err == nil {
				if err := upsertGeneratedSpecInDB(stateDir, string(raw)); err != nil {
					return err
				}
			}
		}

		// Compact bundle
		var hasBundle int
		if err := db.QueryRow(`SELECT COUNT(*) FROM compact_bundle_meta`).Scan(&hasBundle); err != nil {
			return err
		}
		if hasBundle == 0 {
			path := filepath.Join(stateDir, "sessions.compact.json")
			if raw, err := os.ReadFile(path); err == nil {
				var payload compactedSessions
				if json.Unmarshal(raw, &payload) == nil {
					if err := writeCompactBundleToDB(stateDir, payload); err != nil {
						return err
					}
				}
			}
		}

		if _, err := db.Exec(`INSERT INTO state_meta(key, value) VALUES(?, '1')
			ON CONFLICT(key) DO UPDATE SET value='1'`, metaLegacyMigrated); err != nil {
			return err
		}
	}

	var checkpointsMigrated string
	if err := db.QueryRow(`SELECT value FROM state_meta WHERE key = ?`, metaCheckpointsMigrated).Scan(&checkpointsMigrated); err == nil && checkpointsMigrated == "1" {
		return nil
	}
	var checkpointsCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM checkpoints_state`).Scan(&checkpointsCount); err != nil {
		return err
	}
	if checkpointsCount == 0 {
		path := filepath.Join(stateDir, "checkpoints.json")
		if raw, err := os.ReadFile(path); err == nil {
			if err := upsertCheckpointsStateRawInDB(stateDir, string(raw)); err != nil {
				return err
			}
		}
	}
	_, err = db.Exec(`INSERT INTO state_meta(key, value) VALUES(?, '1')
		ON CONFLICT(key) DO UPDATE SET value='1'`, metaCheckpointsMigrated)
	return err
}

func readInteractionTimelineFile(path string) ([]interaction, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	out := make([]interaction, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item interaction
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		if item.SchemaVersion == 0 {
			item.SchemaVersion = 1
		}
		if strings.TrimSpace(item.EventType) == "" {
			if item.InProgress || item.ExitCode < 0 {
				item.EventType = eventTypeUpdate
			} else {
				item.EventType = eventTypeFinal
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func readConversationLogFile(path string) ([]conversationMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var messages []conversationMessage
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, err
	}
	for i := range messages {
		if strings.TrimSpace(messages[i].Dt) == "" {
			messages[i].Dt = time.Now().UTC().Format(time.RFC3339)
		}
	}
	return messages, nil
}
