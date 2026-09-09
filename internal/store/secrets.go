package store

import (
	"database/sql"
	"errors"

	"github.com/jacob-bytes/sounding/internal/api"
)

// GetOrCreateSecret 读取或生成并持久化密钥。
// 用于 -agent-token / -admin-token / -jwt-secret 未显式配置时的默认值，
// 保证重启后 token 不变（否则 systemd/容器重启会导致 Agent 掉线）。
func (s *Store) GetOrCreateSecret(key string) (value string, created bool, err error) {
	var v string
	err = s.db.QueryRow(`SELECT value FROM secrets WHERE key=?`, key).Scan(&v)
	if err == nil && v != "" {
		return v, false, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	v = api.RandomSecret()
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO secrets (key, value) VALUES (?,?)`, key, v); err != nil {
		return "", false, err
	}
	return v, true, nil
}
