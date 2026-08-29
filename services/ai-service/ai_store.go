package main

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type AICredential struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	BaseURL         string `json:"base_url"`
	APIKey          string `json:"api_key,omitempty"`
	APISecret       string `json:"api_secret,omitempty"`
	APIKeyMasked    string `json:"api_key_masked"`
	APISecretMasked string `json:"api_secret_masked,omitempty"`
	HasAPIKey       bool   `json:"has_api_key"`
	HasAPISecret    bool   `json:"has_api_secret"`
	ExtraJSON       string `json:"extra_json"`
	Enabled         bool   `json:"enabled"`
	Source          string `json:"source"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type AIAnalysisRun struct {
	ID             string `json:"id"`
	TaskType       string `json:"task_type"`
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	InputJSON      string `json:"input_json"`
	ResultJSON     string `json:"result_json"`
	Status         string `json:"status"`
	Error          string `json:"error"`
	LatencyMS      int64  `json:"latency_ms"`
	TokenUsageJSON string `json:"token_usage_json"`
	CreatedAt      string `json:"created_at"`
}

func normalizeAICredential(c AICredential) AICredential {
	c.Name = strings.TrimSpace(c.Name)
	c.Provider = normalizeAIProvider(c.Provider)
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.APIKey = strings.TrimSpace(c.APIKey)
	c.APISecret = strings.TrimSpace(c.APISecret)
	c.ExtraJSON = strings.TrimSpace(c.ExtraJSON)
	if c.ExtraJSON == "" {
		c.ExtraJSON = "{}"
	}
	if c.Source == "" {
		c.Source = "stored"
	}
	return c
}

func (s *AppStore) ListAICredentials() ([]AICredential, error) {
	rows, err := s.db.Query(`SELECT id,name,provider,base_url,api_key_encrypted,api_secret_encrypted,extra_json,enabled,created_at,updated_at FROM ai_credentials ORDER BY provider,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AICredential{}
	for rows.Next() {
		var c AICredential
		var apiKeyEncrypted, apiSecretEncrypted string
		var enabled int
		if err := rows.Scan(&c.ID, &c.Name, &c.Provider, &c.BaseURL, &apiKeyEncrypted, &apiSecretEncrypted, &c.ExtraJSON, &enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		apiKey, _ := decryptAISecret(apiKeyEncrypted)
		apiSecret, _ := decryptAISecret(apiSecretEncrypted)
		c.Enabled = intBool(enabled)
		c.Source = "stored"
		c.HasAPIKey = apiKey != ""
		c.HasAPISecret = apiSecret != ""
		c.APIKeyMasked = maskAISecret(apiKey)
		c.APISecretMasked = maskAISecret(apiSecret)
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *AppStore) GetAICredential(id string) (AICredential, error) {
	var c AICredential
	var apiKeyEncrypted, apiSecretEncrypted string
	var enabled int
	err := s.db.QueryRow(`SELECT id,name,provider,base_url,api_key_encrypted,api_secret_encrypted,extra_json,enabled,created_at,updated_at FROM ai_credentials WHERE id=?`, id).
		Scan(&c.ID, &c.Name, &c.Provider, &c.BaseURL, &apiKeyEncrypted, &apiSecretEncrypted, &c.ExtraJSON, &enabled, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, err
	}
	apiKey, err := decryptAISecret(apiKeyEncrypted)
	if err != nil {
		return c, err
	}
	apiSecret, err := decryptAISecret(apiSecretEncrypted)
	if err != nil {
		return c, err
	}
	c.APIKey = apiKey
	c.APISecret = apiSecret
	c.APIKeyMasked = maskAISecret(apiKey)
	c.APISecretMasked = maskAISecret(apiSecret)
	c.HasAPIKey = apiKey != ""
	c.HasAPISecret = apiSecret != ""
	c.Enabled = intBool(enabled)
	c.Source = "stored"
	return c, nil
}

func (s *AppStore) UpsertAICredential(c AICredential) (AICredential, error) {
	c = normalizeAICredential(c)
	if c.Name == "" {
		return c, errors.New("凭据名称不能为空")
	}
	if c.Provider == "" {
		return c, errors.New("provider不能为空")
	}
	if c.APIKey == "" && c.APISecret == "" && c.ID != "" {
		old, err := s.GetAICredential(c.ID)
		if err == nil {
			c.APIKey = old.APIKey
			c.APISecret = old.APISecret
		}
	}
	if c.APIKey == "" {
		return c, errors.New("api_key不能为空")
	}
	if !jsonValidObject(c.ExtraJSON) {
		return c, errors.New("extra_json不是有效JSON对象")
	}

	now := nowText()
	if c.ID == "" {
		c.ID = uuid.NewString()
		c.CreatedAt = now
	} else if c.CreatedAt == "" {
		old, err := s.GetAICredential(c.ID)
		if err == nil {
			c.CreatedAt = old.CreatedAt
		} else if err == sql.ErrNoRows {
			c.CreatedAt = now
		} else {
			return c, err
		}
	}
	c.UpdatedAt = now
	apiKeyEncrypted, err := encryptAISecret(c.APIKey)
	if err != nil {
		return c, err
	}
	apiSecretEncrypted, err := encryptAISecret(c.APISecret)
	if err != nil {
		return c, err
	}
	_, err = s.db.Exec(`INSERT INTO ai_credentials
		(id,name,provider,base_url,api_key_encrypted,api_secret_encrypted,extra_json,enabled,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,provider=excluded.provider,base_url=excluded.base_url,
			api_key_encrypted=excluded.api_key_encrypted,api_secret_encrypted=excluded.api_secret_encrypted,
			extra_json=excluded.extra_json,enabled=excluded.enabled,updated_at=excluded.updated_at`,
		c.ID, c.Name, c.Provider, c.BaseURL, apiKeyEncrypted, apiSecretEncrypted, c.ExtraJSON, boolInt(c.Enabled), c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return c, err
	}
	c.APIKeyMasked = maskAISecret(c.APIKey)
	c.APISecretMasked = maskAISecret(c.APISecret)
	c.HasAPIKey = c.APIKey != ""
	c.HasAPISecret = c.APISecret != ""
	c.APIKey = ""
	c.APISecret = ""
	return c, nil
}

func (s *AppStore) DeleteAICredential(id string) error {
	_, err := s.db.Exec(`DELETE FROM ai_credentials WHERE id=?`, id)
	return err
}

func (s *AppStore) FindEnabledAICredential(provider string) (AICredential, error) {
	provider = normalizeAIProvider(provider)
	var c AICredential
	var apiKeyEncrypted, apiSecretEncrypted string
	var enabled int
	err := s.db.QueryRow(`SELECT id,name,provider,base_url,api_key_encrypted,api_secret_encrypted,extra_json,enabled,created_at,updated_at
		FROM ai_credentials WHERE provider=? AND enabled=1 ORDER BY updated_at DESC LIMIT 1`, provider).
		Scan(&c.ID, &c.Name, &c.Provider, &c.BaseURL, &apiKeyEncrypted, &apiSecretEncrypted, &c.ExtraJSON, &enabled, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, err
	}
	c.APIKey, err = decryptAISecret(apiKeyEncrypted)
	if err != nil {
		return c, err
	}
	c.APISecret, err = decryptAISecret(apiSecretEncrypted)
	if err != nil {
		return c, err
	}
	c.Enabled = intBool(enabled)
	c.Source = "stored"
	return c, nil
}

func (s *AppStore) SaveAIAnalysisRun(run AIAnalysisRun) (AIAnalysisRun, error) {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if strings.TrimSpace(run.InputJSON) == "" {
		run.InputJSON = "{}"
	}
	if strings.TrimSpace(run.ResultJSON) == "" {
		run.ResultJSON = "{}"
	}
	if strings.TrimSpace(run.TokenUsageJSON) == "" {
		run.TokenUsageJSON = "{}"
	}
	if run.CreatedAt == "" {
		run.CreatedAt = nowText()
	}
	_, err := s.db.Exec(`INSERT INTO ai_analysis_runs
		(id,task_type,provider,model,input_json,result_json,status,error,latency_ms,token_usage_json,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		run.ID, run.TaskType, run.Provider, run.Model, run.InputJSON, run.ResultJSON, run.Status, run.Error, run.LatencyMS, run.TokenUsageJSON, run.CreatedAt)
	return run, err
}
