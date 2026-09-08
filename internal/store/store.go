package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// Store 封装 SQLite 存储。
type Store struct {
	db *sql.DB
}

// Open 打开（不存在则创建）SQLite 数据库并进行迁移。
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS nodes (
  uuid TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  cpu_name TEXT NOT NULL DEFAULT '',
  arch TEXT NOT NULL DEFAULT '',
  os TEXT NOT NULL DEFAULT '',
  region TEXT NOT NULL DEFAULT '',
  cpu_cores REAL NOT NULL DEFAULT 1,
  mem_total REAL NOT NULL DEFAULT 0,
  disk_total REAL NOT NULL DEFAULT 0,
  price REAL NOT NULL DEFAULT 0,
  billing_cycle REAL NOT NULL DEFAULT 0,
  currency TEXT NOT NULL DEFAULT 'USD',
  expired_at TEXT NOT NULL DEFAULT '',
  group_name TEXT NOT NULL DEFAULT '',
  tags TEXT NOT NULL DEFAULT '',
  public_remark TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS status_history (
  client TEXT NOT NULL,
  time TEXT NOT NULL,
  cpu REAL NOT NULL DEFAULT 0,
  ram REAL NOT NULL DEFAULT 0,
  swap REAL NOT NULL DEFAULT 0,
  load REAL NOT NULL DEFAULT 0,
  disk REAL NOT NULL DEFAULT 0,
  net_in REAL NOT NULL DEFAULT 0,
  net_out REAL NOT NULL DEFAULT 0,
  net_total_up REAL NOT NULL DEFAULT 0,
  net_total_down REAL NOT NULL DEFAULT 0,
  process REAL NOT NULL DEFAULT 0,
  connections REAL NOT NULL DEFAULT 0,
  connections_udp REAL NOT NULL DEFAULT 0,
  ram_total REAL NOT NULL DEFAULT 0,
  swap_total REAL NOT NULL DEFAULT 0,
  disk_total REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_status_client_time ON status_history(client, time);
`)
	return err
}

// Seed 写入演示节点与状态（供 M1 前端联调）。
func (s *Store) Seed() error {
	nodes := []struct{ uuid, name string }{
		{"demo-001", "Demo 节点 · 本地"},
		{"demo-002", "Demo 节点 · 备用"},
	}
	for i, n := range nodes {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO nodes (uuid, name, cpu_name, arch, os, region, cpu_cores, mem_total, disk_total, price, billing_cycle, currency, expired_at, group_name, tags, public_remark) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			n.uuid, n.name, "Demo CPU", "x86_64", "sounding", "CN", 2, 4<<30, 128<<30, 9.9, 12, "USD", time.Now().AddDate(0, 0, 30).Format(time.RFC3339), "demo", "demo sounding", "内置演示节点"); err != nil {
			return err
		}
		for j := 0; j < 20; j++ {
			ts := time.Now().Add(-time.Duration(20-j) * 30 * time.Second).Format(time.RFC3339)
			cpu := 5 + float64(i)*3 + float64(j%4)
			if _, err := s.db.Exec(`INSERT INTO status_history (client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, ram_total, swap_total, disk_total) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				n.uuid, ts, cpu, 2<<30, 0, 0.8, 40<<30, 3<<20, 2<<20, 10<<30, 5<<30, 120, 40, 6, 4<<30, 2<<30, 128<<30); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }
