package store

import (
	"time"

	"github.com/jacob-bytes/sounding/internal/api"
	"github.com/jacob-bytes/sounding/internal/probe"
)

// Nodes 返回全部节点（含在线/uptime 估算）。
func (s *Store) Nodes() ([]api.NodeRecord, error) {
	rows, err := s.db.Query(`SELECT uuid, name, cpu_name, arch, os, region, cpu_cores, mem_total, disk_total, price, billing_cycle, currency, expired_at, group_name, tags, public_remark FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.NodeRecord
	for rows.Next() {
		var n api.NodeRecord
		if err := rows.Scan(&n.UUID, &n.Name, &n.CPUName, &n.Arch, &n.OS, &n.Region, &n.CPUCores, &n.MemTotal, &n.DiskTotal, &n.Price, &n.BillingCycle, &n.Currency, &n.ExpiredAt, &n.Group, &n.Tags, &n.PublicRemark); err != nil {
			return nil, err
		}
		n.Groups = []string{n.Group}
		n.Online = true
		n.Uptime = 172800
		out = append(out, n)
	}
	return out, rows.Err()
}

// LatestStatus 返回各节点最新一条状态。
func (s *Store) LatestStatus() ([]api.StatusRecord, error) {
	rows, err := s.db.Query(`SELECT client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, ram_total, swap_total, disk_total
		FROM status_history WHERE rowid IN (SELECT MAX(rowid) FROM status_history GROUP BY client) ORDER BY client`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.StatusRecord
	for rows.Next() {
		var r api.StatusRecord
		if err := rows.Scan(&r.Client, &r.Time, &r.CPU, &r.RAM, &r.Swap, &r.Load, &r.Disk, &r.NetIn, &r.NetOut, &r.NetTotalUp, &r.NetTotalDown, &r.Process, &r.Connections, &r.ConnectionsUDP, &r.RAMTotal, &r.SwapTotal, &r.DiskTotal); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecentStatus 返回指定节点最近 limit 条状态（按时间升序）。
func (s *Store) RecentStatus(client string, limit int) ([]api.StatusRecord, error) {
	rows, err := s.db.Query(`SELECT client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, ram_total, swap_total, disk_total
		FROM (SELECT * FROM status_history WHERE client = ? ORDER BY time DESC LIMIT ?) ORDER BY time ASC`, client, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.StatusRecord
	for rows.Next() {
		var r api.StatusRecord
		if err := rows.Scan(&r.Client, &r.Time, &r.CPU, &r.RAM, &r.Swap, &r.Load, &r.Disk, &r.NetIn, &r.NetOut, &r.NetTotalUp, &r.NetTotalDown, &r.Process, &r.Connections, &r.ConnectionsUDP, &r.RAMTotal, &r.SwapTotal, &r.DiskTotal); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}


// InsertStatus 写入一条状态（client = uuid）。
func (s *Store) InsertStatus(uuid string, st api.AgentStatus) error {
	_, err := s.db.Exec(`INSERT INTO status_history (client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, ram_total, swap_total, disk_total)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		uuid, time.Now().Format(time.RFC3339), st.CPU, st.RAM, st.Swap, st.Load, st.Disk, st.NetIn, st.NetOut,
		st.NetTotalUp, st.NetTotalDown, st.Process, st.Connections, st.ConnectionsUDP, st.RAMTotal, st.SwapTotal, st.DiskTotal)
	return err
}

// UpsertNode 注册/更新节点（Agent 首次上报自动注册）。
func (s *Store) UpsertNode(uuid, name, _ string) error {
	_, err := s.db.Exec(`INSERT INTO nodes (uuid, name, cpu_name, arch, os, region, cpu_cores, mem_total, disk_total, price, billing_cycle, currency, expired_at, group_name, tags, public_remark)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(uuid) DO UPDATE SET name=excluded.name`,
		uuid, name, "unknown", "unknown", "sounding-agent", "UNKNOWN", 1, 0, 0, 0, 0, "USD", "", "agent", "", "sounding-agent 节点")
	return err
}

// ProbeTasks 返回启用的探针任务（含全局 + 指定 client）。
func (s *Store) ProbeTasks(client string) ([]probe.Task, error) {
	rows, err := s.db.Query(`SELECT id, client, target, type, name, interval_sec, enabled FROM probe_tasks WHERE client='*' OR client=? ORDER BY id`, client)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []probe.Task
	for rows.Next() {
		var t probe.Task
		var intervalSec float64
		var enabled int
		var cli string
		if err := rows.Scan(&t.ID, &cli, &t.Target, &t.Type, &t.Name, &intervalSec, &enabled); err != nil {
			return nil, err
		}
		t.Interval = time.Duration(intervalSec * float64(time.Second))
		t.Enabled = enabled == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpsertProbeTask 按 (client, name) 创建/更新探针任务。
func (s *Store) UpsertProbeTask(client, target, name, typ string, intervalSec float64, enabled bool) error {
	e := 0
	if enabled {
		e = 1
	}
	_, err := s.db.Exec(`INSERT INTO probe_tasks (client, target, type, name, interval_sec, enabled) VALUES (?,?,?,?,?,?)
		ON CONFLICT(client, name) DO UPDATE SET target=excluded.target, interval_sec=excluded.interval_sec, enabled=excluded.enabled`,
		client, target, typ, name, intervalSec, e)
	return err
}

// InsertProbe 写入探针记录。
func (s *Store) InsertProbe(client string, taskID int, timeStr string, value float64) error {
	_, err := s.db.Exec(`INSERT INTO probe_records (client, task_id, time, value) VALUES (?,?,?,?)`,
		client, taskID, timeStr, value)
	return err
}

// PingRecords 返回指定 client/task 的探针记录（升序，limit 150）。
func (s *Store) PingRecords(client string, taskID int, limit int) ([]api.PingRecord, error) {
	rows, err := s.db.Query(`SELECT client, task_id, time, value FROM (SELECT * FROM probe_records WHERE client=? AND task_id=? ORDER BY time DESC LIMIT ?) ORDER BY time ASC`,
		client, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.PingRecord
	for rows.Next() {
		var r api.PingRecord
		if err := rows.Scan(&r.Client, &r.TaskID, &r.Time, &r.Value); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AdminNodes 管理视角节点列表。
func (s *Store) AdminNodes() ([]api.NodeRecord, error) { return s.Nodes() }

// AdminUpsertNode 手动添加/更新节点。
func (s *Store) AdminUpsertNode(n api.NodeRecord) error {
	_, err := s.db.Exec(`INSERT INTO nodes (uuid, name, cpu_name, arch, os, region, cpu_cores, mem_total, disk_total, price, billing_cycle, currency, expired_at, group_name, tags, public_remark)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(uuid) DO UPDATE SET name=excluded.name, region=excluded.region, price=excluded.price`,
		n.UUID, n.Name, n.CPUName, n.Arch, n.OS, n.Region, n.CPUCores, n.MemTotal, n.DiskTotal, n.Price, n.BillingCycle, n.Currency, n.ExpiredAt, n.Group, n.Tags, n.PublicRemark)
	return err
}

// AdminProbeTasks 管理视角探针任务。
func (s *Store) AdminProbeTasks() ([]api.AdminProbe, error) {
	rows, err := s.db.Query(`SELECT id, client, target, name, enabled FROM probe_tasks ORDER BY client, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.AdminProbe
	for rows.Next() {
		var p api.AdminProbe
		var enabled int
		if err := rows.Scan(&p.ID, &p.Client, &p.Target, &p.Name, &enabled); err != nil {
			return nil, err
		}
		p.Enabled = enabled == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// AdminUpsertProbe 管理视角创建/更新探针。
func (s *Store) AdminUpsertProbe(client, target, name string, enabled bool) error {
	e := 0
	if enabled {
		e = 1
	}
	_, err := s.db.Exec(`INSERT INTO probe_tasks (client, target, name, type, interval_sec, enabled) VALUES (?,?,?,'ping',60,?)
		ON CONFLICT(client, name) DO UPDATE SET target=excluded.target, enabled=excluded.enabled`,
		client, target, name, e)
	return err
}
