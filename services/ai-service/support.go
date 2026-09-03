package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

type AppStore struct {
	db *sql.DB
}

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func successResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(Response{Code: 0, Message: "success", Data: data})
}

func errorResponse(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(Response{Code: -1, Message: message, Data: nil})
}

func withServiceAuth(next http.HandlerFunc) http.HandlerFunc {
	token := strings.TrimSpace(os.Getenv("AI_SERVICE_TOKEN"))
	if token == "" {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimSpace(r.Header.Get("X-AI-Service-Token"))
		if provided == "" {
			provided = strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		}
		if provided != token {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, `{"code":-1,"message":"AI服务令牌无效","data":null}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	status := "healthy"
	if len(startupWarnings) > 0 {
		status = "degraded"
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   status,
		"time":     time.Now().Format(time.RFC3339),
		"warnings": startupWarnings,
	})
}

func handleServerStatus(w http.ResponseWriter, r *http.Request) {
	successResponse(w, map[string]interface{}{
		"status":  "running",
		"ready":   len(startupWarnings) == 0,
		"version": appVersion(),
	})
}

func nowText() string {
	return time.Now().Format(time.RFC3339)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func intBool(value int) bool {
	return value != 0
}

func jsonValidObject(raw string) bool {
	var value map[string]interface{}
	return json.Unmarshal([]byte(raw), &value) == nil
}

func pathParts(path, prefix string) []string {
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func notFoundMessage(err error, fallback string) string {
	if err == sql.ErrNoRows {
		return fallback
	}
	return fmt.Sprintf("%s: %v", fallback, err)
}

func chooseNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func aiDatabasePath() string {
	if value := strings.TrimSpace(os.Getenv("AI_DATABASE_PATH")); value != "" {
		return value
	}
	dataDir := strings.TrimSpace(os.Getenv("AI_DATA_DIR"))
	if dataDir == "" {
		dataDir = "./data/database"
	}
	return filepath.Join(dataDir, "ai.db")
}

func OpenAppStore() (*AppStore, error) {
	path := aiDatabasePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &AppStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *AppStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *AppStore) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ai_credentials (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
				provider TEXT NOT NULL,
				base_url TEXT NOT NULL DEFAULT '',
				model TEXT NOT NULL DEFAULT '',
				api_key_encrypted TEXT NOT NULL DEFAULT '',
				api_secret_encrypted TEXT NOT NULL DEFAULT '',
				extra_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_credentials_provider ON ai_credentials(provider)`,
		`CREATE TABLE IF NOT EXISTS ai_analysis_runs (
			id TEXT PRIMARY KEY,
			task_type TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			input_json TEXT NOT NULL DEFAULT '{}',
			result_json TEXT NOT NULL DEFAULT '{}',
			status TEXT NOT NULL,
			error TEXT NOT NULL DEFAULT '',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			token_usage_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := s.ensureColumn("ai_credentials", "model", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	for _, column := range []struct{ name, definition string }{
		{"prompt_version", "TEXT NOT NULL DEFAULT 'v1'"},
		{"data_revision", "TEXT NOT NULL DEFAULT ''"},
		{"tools_json", "TEXT NOT NULL DEFAULT '[]'"},
	} {
		if err := s.ensureColumn("ai_analysis_runs", column.name, column.definition); err != nil {
			return err
		}
	}
	return nil
}

func (s *AppStore) ensureColumn(table, column, definition string) error {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return err
}

func requireNonEmpty(value, message string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New(message)
	}
	return nil
}
