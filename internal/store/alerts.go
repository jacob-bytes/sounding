package store

import (
	"encoding/json"

	"github.com/jacob-bytes/sounding/internal/alert"
)

// LoadAlertRules 读取持久化的告警规则。
func (s *Store) LoadAlertRules() ([]alert.Rule, error) {
	rows, err := s.db.Query(`SELECT kind, node, threshold, silence_until, mute_windows FROM alert_rules ORDER BY kind, node`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []alert.Rule
	for rows.Next() {
		var r alert.Rule
		var mute string
		if err := rows.Scan(&r.Kind, &r.Node, &r.Threshold, &r.SilenceUntil, &mute); err != nil {
			return nil, err
		}
		if mute != "" {
			_ = json.Unmarshal([]byte(mute), &r.MuteWindows)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SaveAlertRule 新增/更新告警规则（同 kind+node 覆盖）。
func (s *Store) SaveAlertRule(r alert.Rule) error {
	mute, _ := json.Marshal(r.MuteWindows)
	_, err := s.db.Exec(`INSERT INTO alert_rules (kind, node, threshold, silence_until, mute_windows) VALUES (?,?,?,?,?)
		ON CONFLICT(kind, node) DO UPDATE SET threshold=excluded.threshold, silence_until=excluded.silence_until, mute_windows=excluded.mute_windows`,
		r.Kind, r.Node, r.Threshold, r.SilenceUntil, string(mute))
	return err
}

// DeleteAlertRule 删除告警规则。
func (s *Store) DeleteAlertRule(kind, node string) error {
	_, err := s.db.Exec(`DELETE FROM alert_rules WHERE kind=? AND node=?`, kind, node)
	return err
}
