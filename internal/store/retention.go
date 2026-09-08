package store

import (
	"context"
	"log"
	"time"
)

// Purge 清理超过 retainDays 的历史数据（status_history / probe_records）。
func (s *Store) Purge(retainDays int) (int64, error) {
	if retainDays <= 0 {
		retainDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -retainDays).Format(time.RFC3339)
	res, err := s.db.Exec(`DELETE FROM status_history WHERE time < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	res2, err := s.db.Exec(`DELETE FROM probe_records WHERE time < ?`, cutoff)
	if err != nil {
		return n, err
	}
	n2, _ := res2.RowsAffected()
	return n + n2, nil
}

// Vacuum 回收空间（定期执行）。
func (s *Store) Vacuum() error {
	_, err := s.db.Exec(`VACUUM`)
	return err
}

// RetentionLoop 周期清理（每 6 小时）。
func (s *Store) RetentionLoop(ctx context.Context, retainDays int) {
	t := time.NewTicker(6 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n, err := s.Purge(retainDays)
			if err != nil {
				log.Printf("retention: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("retention: purged %d rows (retain %d days)", n, retainDays)
				if err := s.Vacuum(); err != nil {
					log.Printf("retention vacuum: %v", err)
				}
			}
		}
	}
}
