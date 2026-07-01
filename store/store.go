package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"transbridge/config"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Token struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Token        string     `json:"token"`
	Scope        string     `json:"scope"`
	Enabled      bool       `json:"enabled"`
	RequestCount int64      `json:"request_count"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type TokenView struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Token        string     `json:"token"`
	Scope        string     `json:"scope"`
	Enabled      bool       `json:"enabled"`
	RequestCount int64      `json:"request_count"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type PromptVersion struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Template  string    `json:"template"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type RequestLog struct {
	ID            int64     `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	Endpoint      string    `json:"endpoint"`
	SourceLang    string    `json:"source_lang"`
	TargetLang    string    `json:"target_lang"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	APIURL        string    `json:"api_url"`
	CacheHit      bool      `json:"cache_hit"`
	Success       bool      `json:"success"`
	Error         string    `json:"error,omitempty"`
	ProcessTimeMS float64   `json:"process_time_ms"`
	SourceChars   int       `json:"source_chars"`
	TargetChars   int       `json:"target_chars"`
}

type Stats struct {
	Requests       int64   `json:"requests"`
	Successes      int64   `json:"successes"`
	Failures       int64   `json:"failures"`
	CacheHits      int64   `json:"cache_hits"`
	AvgLatencyMS   float64 `json:"avg_latency_ms"`
	Models         int64   `json:"models"`
	EnabledTokens  int64   `json:"enabled_tokens"`
	PromptVersions int64   `json:"prompt_versions"`
}

type ModelRecord struct {
	ID                int64    `json:"id"`
	ProviderID        int64    `json:"provider_id"`
	Provider          string   `json:"provider"`
	APIURL            string   `json:"api_url"`
	APIKey            string   `json:"api_key"`
	ProviderTimeout   int      `json:"provider_timeout"`
	IsDefault         bool     `json:"is_default"`
	Name              string   `json:"name"`
	Weight            int      `json:"weight"`
	TopP              int      `json:"top_p"`
	MaxTokens         int      `json:"max_tokens"`
	Temperature       float32  `json:"temperature"`
	Timeout           *int     `json:"timeout,omitempty"`
	Enabled           bool     `json:"enabled"`
	RateLimit         RateSpec `json:"rate_limit"`
	ProviderRateLimit RateSpec `json:"provider_rate_limit"`
}

type ModelView struct {
	ID                int64    `json:"id"`
	ProviderID        int64    `json:"provider_id"`
	Provider          string   `json:"provider"`
	APIURL            string   `json:"api_url"`
	APIKey            string   `json:"api_key"`
	ProviderTimeout   int      `json:"provider_timeout"`
	IsDefault         bool     `json:"is_default"`
	Name              string   `json:"name"`
	Weight            int      `json:"weight"`
	TopP              int      `json:"top_p"`
	MaxTokens         int      `json:"max_tokens"`
	Temperature       float32  `json:"temperature"`
	Timeout           *int     `json:"timeout,omitempty"`
	Enabled           bool     `json:"enabled"`
	RateLimit         RateSpec `json:"rate_limit"`
	ProviderRateLimit RateSpec `json:"provider_rate_limit"`
}

type RateSpec struct {
	MaxConcurrent int `json:"max_concurrent"`
	QPS           int `json:"qps"`
	QPM           int `json:"qpm"`
}

func Open(path string) (*Store, error) {
	if path == "" {
		path = "data/transbridge.db"
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create sqlite directory: %w", err)
		}
	}
	// modernc.org/sqlite DSN 用 _pragma=name(val) 语法（不同于 mattn/go-sqlite3 的 _foo=val）。
	// 每一个新连接打开时都会自动应用这些 pragma——因此连接池里所有 connection 都进入 WAL 模式。
	//   journal_mode=WAL   多并发写者共享 WAL 文件，读不阻塞写、写不阻塞读
	//   busy_timeout=5000  拿不到锁时驱动内部轮询重试 5 秒，避免立刻 SQLITE_BUSY
	//   synchronous=NORMAL 配合 WAL 时数据安全 & 比 FULL 快数倍
	dsn := path
	if !strings.Contains(dsn, "?") {
		dsn += "?"
	} else {
		dsn += "&"
	}
	dsn += "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// 保留多连接：WAL 允许多读者并发，busy_timeout 让写者排队等锁；
	// 若强制 SetMaxOpenConns(1)，会与 LoadProviders 里嵌套 Query 死锁
	// （外层 rows 还占着唯一连接时，for 循环里对同一 db 发第二个 Query 拿不到连接）。
	db.SetMaxOpenConns(4)
	s := &Store{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS providers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			api_url TEXT NOT NULL,
			api_key TEXT NOT NULL DEFAULT '',
			timeout INTEGER NOT NULL DEFAULT 30,
			is_default INTEGER NOT NULL DEFAULT 0,
			rate_max_concurrent INTEGER NOT NULL DEFAULT 0,
			rate_qps INTEGER NOT NULL DEFAULT 0,
			rate_qpm INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS models (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_id INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			weight INTEGER NOT NULL DEFAULT 1,
			top_p INTEGER NOT NULL DEFAULT 0,
			max_tokens INTEGER NOT NULL DEFAULT 2000,
			temperature REAL NOT NULL DEFAULT 0.3,
			timeout INTEGER,
			rate_max_concurrent INTEGER NOT NULL DEFAULT 0,
			rate_qps INTEGER NOT NULL DEFAULT 0,
			rate_qpm INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(provider_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			token TEXT NOT NULL UNIQUE,
			scope TEXT NOT NULL DEFAULT 'translate',
			enabled INTEGER NOT NULL DEFAULT 1,
			request_count INTEGER NOT NULL DEFAULT 0,
			last_used_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS prompt_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			template TEXT NOT NULL,
			active INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT NOT NULL,
			endpoint TEXT NOT NULL DEFAULT '',
			source_lang TEXT NOT NULL DEFAULT '',
			target_lang TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			api_url TEXT NOT NULL DEFAULT '',
			cache_hit INTEGER NOT NULL DEFAULT 0,
			success INTEGER NOT NULL DEFAULT 1,
			error TEXT NOT NULL DEFAULT '',
			process_time_ms REAL NOT NULL DEFAULT 0,
			source_chars INTEGER NOT NULL DEFAULT 0,
			target_chars INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_model ON request_logs(provider, model)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) BootstrapFromConfig(ctx context.Context, cfg *config.Config) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM providers`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if err := s.SaveProviders(ctx, cfg.Providers); err != nil {
			return err
		}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tokens`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		for _, token := range cfg.TransAPI.Tokens {
			if token == "" {
				continue
			}
			if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO tokens(name, token, scope, enabled, created_at, updated_at) VALUES(?, ?, 'translate', 1, ?, ?)`, "config token", token, now, now); err != nil {
				return err
			}
		}
		for _, token := range cfg.OpenAI.CompatibleAPI.AuthTokens {
			if token == "" {
				continue
			}
			if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO tokens(name, token, scope, enabled, created_at, updated_at) VALUES(?, ?, 'openai', 1, ?, ?)`, "openai token", token, now, now); err != nil {
				return err
			}
		}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompt_versions`).Scan(&count); err != nil {
		return err
	}
	if count == 0 && cfg.Prompt.Template != "" {
		return s.CreatePrompt(ctx, PromptVersion{Name: "config prompt", Template: cfg.Prompt.Template, Active: true})
	}
	return nil
}

func (s *Store) SaveProviders(ctx context.Context, providers []config.ProviderConfig) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM models`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM providers`); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, p := range providers {
		res, err := tx.ExecContext(ctx, `INSERT INTO providers(provider, api_url, api_key, timeout, is_default, rate_max_concurrent, rate_qps, rate_qpm, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Provider, p.APIURL, p.APIKey, p.Timeout, boolInt(p.IsDefault), p.RateLimit.MaxConcurrent, p.RateLimit.QPS, p.RateLimit.QPM, now, now)
		if err != nil {
			return err
		}
		providerID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for _, m := range p.Models {
			var timeout any
			if m.Timeout != nil {
				timeout = *m.Timeout
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO models(provider_id, name, weight, top_p, max_tokens, temperature, timeout, rate_max_concurrent, rate_qps, rate_qpm, enabled, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
				providerID, m.Name, m.Weight, m.TopP, m.MaxTokens, m.Temperature, timeout, m.RateLimit.MaxConcurrent, m.RateLimit.QPS, m.RateLimit.QPM, now, now); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) LoadProviders(ctx context.Context) ([]config.ProviderConfig, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, provider, api_url, api_key, timeout, is_default, rate_max_concurrent, rate_qps, rate_qpm FROM providers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := make([]config.ProviderConfig, 0)
	for rows.Next() {
		var id int64
		var isDefault int
		var p config.ProviderConfig
		if err := rows.Scan(&id, &p.Provider, &p.APIURL, &p.APIKey, &p.Timeout, &isDefault, &p.RateLimit.MaxConcurrent, &p.RateLimit.QPS, &p.RateLimit.QPM); err != nil {
			return nil, err
		}
		p.IsDefault = isDefault == 1
		modelRows, err := s.db.QueryContext(ctx, `SELECT name, weight, top_p, max_tokens, temperature, timeout, rate_max_concurrent, rate_qps, rate_qpm FROM models WHERE provider_id = ? AND enabled = 1 ORDER BY id`, id)
		if err != nil {
			return nil, err
		}
		for modelRows.Next() {
			var m config.ModelConfig
			var timeout sql.NullInt64
			if err := modelRows.Scan(&m.Name, &m.Weight, &m.TopP, &m.MaxTokens, &m.Temperature, &timeout, &m.RateLimit.MaxConcurrent, &m.RateLimit.QPS, &m.RateLimit.QPM); err != nil {
				modelRows.Close()
				return nil, err
			}
			if timeout.Valid {
				v := int(timeout.Int64)
				m.Timeout = &v
			}
			p.Models = append(p.Models, m)
		}
		if err := modelRows.Close(); err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

func (s *Store) ListModels(ctx context.Context) ([]ModelRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT m.id, p.id, p.provider, p.api_url, p.api_key, p.timeout, p.is_default, p.rate_max_concurrent, p.rate_qps, p.rate_qpm, m.name, m.weight, m.top_p, m.max_tokens, m.temperature, m.timeout, m.rate_max_concurrent, m.rate_qps, m.rate_qpm, m.enabled
		FROM models m JOIN providers p ON p.id = m.provider_id ORDER BY p.id, m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]ModelRecord, 0)
	for rows.Next() {
		var r ModelRecord
		var providerDefault, enabled int
		var timeout sql.NullInt64
		if err := rows.Scan(&r.ID, &r.ProviderID, &r.Provider, &r.APIURL, &r.APIKey, &r.ProviderTimeout, &providerDefault, &r.ProviderRateLimit.MaxConcurrent, &r.ProviderRateLimit.QPS, &r.ProviderRateLimit.QPM, &r.Name, &r.Weight, &r.TopP, &r.MaxTokens, &r.Temperature, &timeout, &r.RateLimit.MaxConcurrent, &r.RateLimit.QPS, &r.RateLimit.QPM, &enabled); err != nil {
			return nil, err
		}
		r.IsDefault = providerDefault == 1
		r.Enabled = enabled == 1
		if timeout.Valid {
			v := int(timeout.Int64)
			r.Timeout = &v
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *Store) ListModelViews(ctx context.Context) ([]ModelView, error) {
	models, err := s.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]ModelView, 0, len(models))
	for _, model := range models {
		views = append(views, ModelView{
			ID:                model.ID,
			ProviderID:        model.ProviderID,
			Provider:          model.Provider,
			APIURL:            model.APIURL,
			APIKey:            maskSecret(model.APIKey),
			ProviderTimeout:   model.ProviderTimeout,
			IsDefault:         model.IsDefault,
			Name:              model.Name,
			Weight:            model.Weight,
			TopP:              model.TopP,
			MaxTokens:         model.MaxTokens,
			Temperature:       model.Temperature,
			Timeout:           model.Timeout,
			Enabled:           model.Enabled,
			RateLimit:         model.RateLimit,
			ProviderRateLimit: model.ProviderRateLimit,
		})
	}
	return views, nil
}

func (s *Store) EnabledModelCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM models WHERE enabled = 1`).Scan(&count)
	return count, err
}

func (s *Store) UpsertProviderModel(ctx context.Context, p config.ProviderConfig, m config.ModelConfig, enabled bool) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var providerID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM providers WHERE provider = ? AND api_url = ?`, p.Provider, p.APIURL).Scan(&providerID)
	if errors.Is(err, sql.ErrNoRows) {
		res, err := tx.ExecContext(ctx, `INSERT INTO providers(provider, api_url, api_key, timeout, is_default, rate_max_concurrent, rate_qps, rate_qpm, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Provider, p.APIURL, p.APIKey, p.Timeout, boolInt(p.IsDefault), p.RateLimit.MaxConcurrent, p.RateLimit.QPS, p.RateLimit.QPM, now, now)
		if err != nil {
			return err
		}
		providerID, err = res.LastInsertId()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if p.APIKey == "" {
			if err := tx.QueryRowContext(ctx, `SELECT api_key FROM providers WHERE id = ?`, providerID).Scan(&p.APIKey); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE providers SET api_key = ?, timeout = ?, is_default = ?, rate_max_concurrent = ?, rate_qps = ?, rate_qpm = ?, updated_at = ? WHERE id = ?`,
			p.APIKey, p.Timeout, boolInt(p.IsDefault), p.RateLimit.MaxConcurrent, p.RateLimit.QPS, p.RateLimit.QPM, now, providerID); err != nil {
			return err
		}
	}

	var timeout any
	if m.Timeout != nil {
		timeout = *m.Timeout
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO models(provider_id, name, weight, top_p, max_tokens, temperature, timeout, rate_max_concurrent, rate_qps, rate_qpm, enabled, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_id, name) DO UPDATE SET weight = excluded.weight, top_p = excluded.top_p, max_tokens = excluded.max_tokens, temperature = excluded.temperature, timeout = excluded.timeout, rate_max_concurrent = excluded.rate_max_concurrent, rate_qps = excluded.rate_qps, rate_qpm = excluded.rate_qpm, enabled = excluded.enabled, updated_at = excluded.updated_at`,
		providerID, m.Name, m.Weight, m.TopP, m.MaxTokens, m.Temperature, timeout, m.RateLimit.MaxConcurrent, m.RateLimit.QPS, m.RateLimit.QPM, boolInt(enabled), now, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteModel(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM models WHERE id = ?`, id)
	return err
}

// SetModelEnabled 只翻转 enabled 字段，不动其它任何配置。
// 供 /admin/api/models/toggle 使用，一键启用/禁用。
func (s *Store) SetModelEnabled(ctx context.Context, id int64, enabled bool) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `UPDATE models SET enabled = ?, updated_at = ? WHERE id = ?`, boolInt(enabled), now, id)
	return err
}

// GetModelByID 拉一条 model 详情用于状态回读（当前只用于 toggle handler 校验存在性与获取当前 enabled）。
func (s *Store) GetModelByID(ctx context.Context, id int64) (bool, error) {
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT enabled FROM models WHERE id = ?`, id).Scan(&enabled)
	if err != nil {
		return false, err
	}
	return enabled == 1, nil
}

func (s *Store) DeleteModelByName(ctx context.Context, provider, apiURL, model string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM models WHERE name = ? AND provider_id IN (SELECT id FROM providers WHERE provider = ? AND api_url = ?)`, model, provider, apiURL)
	return err
}

func (s *Store) ListTokens(ctx context.Context) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, token, scope, enabled, request_count, last_used_at, created_at, updated_at FROM tokens ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens := make([]Token, 0)
	for rows.Next() {
		var t Token
		var enabled int
		var last sql.NullString
		var created, updated string
		if err := rows.Scan(&t.ID, &t.Name, &t.Token, &t.Scope, &enabled, &t.RequestCount, &last, &created, &updated); err != nil {
			return nil, err
		}
		t.Enabled = enabled == 1
		t.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		if last.Valid {
			parsed, _ := time.Parse(time.RFC3339Nano, last.String)
			t.LastUsedAt = &parsed
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (s *Store) ListTokenViews(ctx context.Context) ([]TokenView, error) {
	tokens, err := s.ListTokens(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]TokenView, 0, len(tokens))
	for _, token := range tokens {
		views = append(views, TokenView{
			ID:           token.ID,
			Name:         token.Name,
			Token:        maskSecret(token.Token),
			Scope:        token.Scope,
			Enabled:      token.Enabled,
			RequestCount: token.RequestCount,
			LastUsedAt:   token.LastUsedAt,
			CreatedAt:    token.CreatedAt,
			UpdatedAt:    token.UpdatedAt,
		})
	}
	return views, nil
}

func (s *Store) GetTokenByID(ctx context.Context, id int64) (*Token, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, token, scope, enabled, request_count, last_used_at, created_at, updated_at FROM tokens WHERE id = ?`, id)
	var t Token
	var enabled int
	var last sql.NullString
	var created, updated string
	if err := row.Scan(&t.ID, &t.Name, &t.Token, &t.Scope, &enabled, &t.RequestCount, &last, &created, &updated); err != nil {
		return nil, err
	}
	t.Enabled = enabled == 1
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	if last.Valid {
		parsed, _ := time.Parse(time.RFC3339Nano, last.String)
		t.LastUsedAt = &parsed
	}
	return &t, nil
}

func (s *Store) CreateToken(ctx context.Context, t Token) error {
	if t.Scope == "" {
		t.Scope = "translate"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO tokens(name, token, scope, enabled, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)`, t.Name, t.Token, t.Scope, boolInt(t.Enabled), now, now)
	return err
}

func (s *Store) UpdateToken(ctx context.Context, t Token) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `UPDATE tokens SET name = ?, token = ?, scope = ?, enabled = ?, updated_at = ? WHERE id = ?`, t.Name, t.Token, t.Scope, boolInt(t.Enabled), now, t.ID)
	return err
}

func (s *Store) DeleteToken(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tokens WHERE id = ?`, id)
	return err
}

func (s *Store) TokenAllowed(ctx context.Context, token string, scopes ...string) (bool, error) {
	if token == "" {
		return false, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT scope FROM tokens WHERE token = ? AND enabled = 1`, token)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	allowedScopes := map[string]bool{}
	for _, scope := range scopes {
		allowedScopes[scope] = true
	}
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			return false, err
		}
		if len(allowedScopes) == 0 || allowedScopes[scope] || scope == "all" {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) MarkTokenUsed(ctx context.Context, token string) {
	if token == "" {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = s.db.ExecContext(ctx, `UPDATE tokens SET request_count = request_count + 1, last_used_at = ?, updated_at = ? WHERE token = ?`, now, now, token)
}

func (s *Store) ActivePrompt(ctx context.Context, fallback string) (string, error) {
	var template string
	err := s.db.QueryRowContext(ctx, `SELECT template FROM prompt_versions WHERE active = 1 ORDER BY id DESC LIMIT 1`).Scan(&template)
	if errors.Is(err, sql.ErrNoRows) {
		return fallback, nil
	}
	return template, err
}

func (s *Store) ListPrompts(ctx context.Context) ([]PromptVersion, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, template, active, created_at FROM prompt_versions ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	prompts := make([]PromptVersion, 0)
	for rows.Next() {
		var p PromptVersion
		var active int
		var created string
		if err := rows.Scan(&p.ID, &p.Name, &p.Template, &active, &created); err != nil {
			return nil, err
		}
		p.Active = active == 1
		p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		prompts = append(prompts, p)
	}
	return prompts, rows.Err()
}

func (s *Store) CreatePrompt(ctx context.Context, p PromptVersion) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if p.Active {
		if _, err := tx.ExecContext(ctx, `UPDATE prompt_versions SET active = 0`); err != nil {
			return err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO prompt_versions(name, template, active, created_at) VALUES(?, ?, ?, ?)`, p.Name, p.Template, boolInt(p.Active), now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ActivatePrompt(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE prompt_versions SET active = 0`); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE prompt_versions SET active = 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (s *Store) LogRequest(ctx context.Context, r RequestLog) error {
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO request_logs(timestamp, endpoint, source_lang, target_lang, provider, model, api_url, cache_hit, success, error, process_time_ms, source_chars, target_chars) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Timestamp.UTC().Format(time.RFC3339Nano), r.Endpoint, r.SourceLang, r.TargetLang, r.Provider, r.Model, r.APIURL, boolInt(r.CacheHit), boolInt(r.Success), r.Error, r.ProcessTimeMS, r.SourceChars, r.TargetChars)
	return err
}

func (s *Store) ListRequestLogs(ctx context.Context, limit int) ([]RequestLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, timestamp, endpoint, source_lang, target_lang, provider, model, api_url, cache_hit, success, error, process_time_ms, source_chars, target_chars FROM request_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := make([]RequestLog, 0)
	for rows.Next() {
		var r RequestLog
		var ts string
		var cacheHit, success int
		if err := rows.Scan(&r.ID, &ts, &r.Endpoint, &r.SourceLang, &r.TargetLang, &r.Provider, &r.Model, &r.APIURL, &cacheHit, &success, &r.Error, &r.ProcessTimeMS, &r.SourceChars, &r.TargetChars); err != nil {
			return nil, err
		}
		r.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		r.CacheHit = cacheHit == 1
		r.Success = success == 1
		logs = append(logs, r)
	}
	return logs, rows.Err()
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var stats Stats
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(success), 0), COALESCE(SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END), 0), COALESCE(SUM(cache_hit), 0), COALESCE(AVG(process_time_ms), 0) FROM request_logs`).
		Scan(&stats.Requests, &stats.Successes, &stats.Failures, &stats.CacheHits, &stats.AvgLatencyMS)
	if err != nil {
		return stats, err
	}
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM models WHERE enabled = 1`).Scan(&stats.Models)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tokens WHERE enabled = 1`).Scan(&stats.EnabledTokens)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompt_versions`).Scan(&stats.PromptVersions)
	return stats, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "..." + value[len(value)-4:]
}
