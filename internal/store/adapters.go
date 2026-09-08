package store

import (
	"fmt"
	"time"

	"github.com/jacob-bytes/sounding/internal/alert"
	"github.com/jacob-bytes/sounding/internal/api"
	"github.com/jacob-bytes/sounding/internal/probe"
)

// offlineAfter 超过该时长无上报视为离线。
const offlineAfter = 60 * time.Second

// Nodes 返回 map[uuid]Client（ink 契约）。
func (s *Store) Nodes() (map[string]api.Client, error) {
	rows, err := s.db.Query(`SELECT uuid, name, cpu_name, arch, os, region, cpu_cores, mem_total, disk_total, price, billing_cycle, currency, expired_at, group_name, tags, public_remark FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]api.Client{}
	for rows.Next() {
		var c api.Client
		if err := rows.Scan(&c.UUID, &c.Name, &c.CPUName, &c.Arch, &c.OS, &c.Region, &c.CPUCores, &c.MemTotal, &c.DiskTotal, &c.Price, &c.BillingCycle, &c.Currency, &c.ExpiredAt, &c.Group, &c.Tags, &c.PublicRemark); err != nil {
			return nil, err
		}
		c.Groups = []string{c.Group}
		out[c.UUID] = c
	}
	return out, rows.Err()
}

// LatestStatus 返回 map[uuid]NodeStatus（含 online/uptime/ping 汇总）。
func (s *Store) LatestStatus() (map[string]api.NodeStatus, error) {
	rows, err := s.db.Query(`SELECT client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, ram_total, swap_total, disk_total, temp, load5, load15
		FROM status_history WHERE rowid IN (SELECT MAX(rowid) FROM status_history GROUP BY client)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]api.NodeStatus{}
	for rows.Next() {
		var n api.NodeStatus
		if err := rows.Scan(&n.Client, &n.Time, &n.CPU, &n.RAM, &n.Swap, &n.Load, &n.Disk, &n.NetIn, &n.NetOut, &n.NetTotalUp, &n.NetTotalDown, &n.Process, &n.Connections, &n.ConnectionsUDP, &n.RAMTotal, &n.SwapTotal, &n.DiskTotal, &n.Temp, &n.Load5, &n.Load15); err != nil {
			return nil, err
		}
		out[n.Client] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 在线/uptime + ping 汇总
	for uuid, n := range out {
		n.Online, n.Uptime = s.onlineState(uuid, n.Time)
		n.Ping = s.pingSummary(uuid)
		out[uuid] = n
	}
	return out, nil
}

// onlineState 根据最后上报时间判断在线与运行时长。
func (s *Store) onlineState(uuid, lastTime string) (bool, float64) {
	t, err := time.Parse(time.RFC3339, lastTime)
	if err != nil {
		return false, 0
	}
	online := time.Since(t) < offlineAfter
	// 首条上报时间作为启动点（近似 uptime）
	var first string
	_ = s.db.QueryRow(`SELECT time FROM status_history WHERE client=? ORDER BY time ASC LIMIT 1`, uuid).Scan(&first)
	var uptime float64
	if ft, err := time.Parse(time.RFC3339, first); err == nil {
		uptime = time.Since(ft).Seconds()
	}
	return online, uptime
}

// pingSummary 聚合每个探针任务的最新/均值/丢包（ink 契约）。
func (s *Store) pingSummary(uuid string) map[string]api.NodeStatusPing {
	rows, err := s.db.Query(`SELECT t.id, t.name, p.value, p.time FROM probe_tasks t
		LEFT JOIN probe_records p ON p.task_id = t.id AND p.client = ?
		WHERE t.client = '*' OR t.client = ?`, uuid, uuid)
	if err != nil {
		return nil
	}
	defer rows.Close()
	agg := map[int]*api.NodeStatusPing{}
	vals := map[int][]float64{}
	names := map[int]string{}
	for rows.Next() {
		var id int
		var name string
		var val *float64
		var ts *string
		if err := rows.Scan(&id, &name, &val, &ts); err != nil {
			continue
		}
		names[id] = name
		if val != nil {
			vals[id] = append(vals[id], *val)
		}
	}
	out := map[string]api.NodeStatusPing{}
	for id, name := range names {
		vs := vals[id]
		p := api.NodeStatusPing{Name: name}
		if len(vs) > 0 {
			sum, minV, maxV := 0.0, vs[0], vs[0]
			for _, v := range vs {
				sum += v
				if v < minV {
					minV = v
				}
				if v > maxV {
					maxV = v
				}
			}
			p.Latest = vs[len(vs)-1]
			p.Avg = sum / float64(len(vs))
			p.Tail = vs[len(vs)-1]
			p.Min, p.Max = minV, maxV
		} else {
			p.Latest = -1
			p.Loss = 100
		}
		out[fmt.Sprint(id)] = p
		agg[id] = &p
	}
	return out
}

// RecentStatus 返回 {count, records}（ink 契约）。
func (s *Store) RecentStatus(client string, limit int) (map[string]any, error) {
	rows, err := s.db.Query(`SELECT client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, ram_total, swap_total, disk_total
		FROM (SELECT * FROM status_history WHERE client = ? ORDER BY time DESC LIMIT ?) ORDER BY time ASC`, client, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var recs []api.StatusRecord
	for rows.Next() {
		var r api.StatusRecord
		if err := rows.Scan(&r.Client, &r.Time, &r.CPU, &r.RAM, &r.Swap, &r.Load, &r.Disk, &r.NetIn, &r.NetOut, &r.NetTotalUp, &r.NetTotalDown, &r.Process, &r.Connections, &r.ConnectionsUDP, &r.RAMTotal, &r.SwapTotal, &r.DiskTotal); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	return map[string]any{"count": len(recs), "records": recs}, rows.Err()
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
		var cli string
		var intervalSec float64
		var enabled int
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
	_, err := s.db.Exec(`INSERT INTO probe_records (client, task_id, time, value) VALUES (?,?,?,?)`, client, taskID, timeStr, value)
	return err
}

// PingRecords 返回探针记录（升序）。
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

// UpsertNode 注册/更新节点（Agent 首次上报自动注册）。
func (s *Store) UpsertNode(uuid, name, info string) error {
	_, err := s.db.Exec(`INSERT INTO nodes (uuid, name, cpu_name, arch, os, region, cpu_cores, mem_total, disk_total, price, billing_cycle, currency, expired_at, group_name, tags, public_remark)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(uuid) DO UPDATE SET name=excluded.name`,
		uuid, name, "unknown", "unknown", "sounding-agent", "UNKNOWN", 1, 0, 0, 0, 0, "USD", "", "agent", "", "sounding-agent 节点")
	return err
}

// InsertStatus 写入一条状态（client = uuid）。
func (s *Store) InsertStatus(uuid string, st api.AgentStatus) error {
	_, err := s.db.Exec(`INSERT INTO status_history (client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, temp, load5, load15, ram_total, swap_total, disk_total)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		uuid, time.Now().Format(time.RFC3339), st.CPU, st.RAM, st.Swap, st.Load, st.Disk, st.NetIn, st.NetOut,
		st.NetTotalUp, st.NetTotalDown, st.Process, st.Connections, st.ConnectionsUDP, st.Temp, st.Load5, st.Load15, st.RAMTotal, st.SwapTotal, st.DiskTotal)
	return err
}

// AdminNodes 管理视角节点列表。
func (s *Store) AdminNodes() (map[string]api.Client, error) { return s.Nodes() }

// AdminUpsertNode 手动添加/更新节点。
func (s *Store) AdminUpsertNode(n api.Client) error {
	_, err := s.db.Exec(`INSERT INTO nodes (uuid, name, cpu_name, arch, os, region, cpu_cores, mem_total, disk_total, price, billing_cycle, currency, expired_at, group_name, tags, public_remark)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(uuid) DO UPDATE SET name=excluded.name, region=excluded.region, price=excluded.price`,
		n.UUID, n.Name, n.CPUName, n.Arch, n.OS, n.Region, n.CPUCores, n.MemTotal, n.DiskTotal, n.Price, n.BillingCycle, n.Currency, n.ExpiredAt, n.Group, n.Tags, n.PublicRemark)
	return err
}

// AdminProbeTasks 管理视角探针任务。
func (s *Store) AdminProbeTasks() ([]api.AdminProbe, error) {
	rows, err := s.db.Query(`SELECT id, client, target, name, type, enabled FROM probe_tasks ORDER BY client, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.AdminProbe
	for rows.Next() {
		var p api.AdminProbe
		var enabled int
		if err := rows.Scan(&p.ID, &p.Client, &p.Target, &p.Name, &p.Type, &enabled); err != nil {
			return nil, err
		}
		p.Enabled = enabled == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// AdminUpsertProbe 管理视角创建/更新探针（typ: icmp|tcp|http|dns）。
func (s *Store) AdminUpsertProbe(client, target, name, typ string, enabled bool) error {
	if typ == "" {
		typ = "icmp"
	}
	e := 0
	if enabled {
		e = 1
	}
	_, err := s.db.Exec(`INSERT INTO probe_tasks (client, target, name, type, interval_sec, enabled) VALUES (?,?,?,?,60,?)
		ON CONFLICT(client, name) DO UPDATE SET target=excluded.target, type=excluded.type, enabled=excluded.enabled`,
		client, target, name, typ, e)
	return err
}

// AlertSnapshots 提供告警判定所需的节点快照。
func (s *Store) AlertSnapshots() ([]alert.Snapshot, error) {
	statuses, err := s.LatestStatus()
	if err != nil {
		return nil, err
	}
	clients, err := s.Nodes()
	if err != nil {
		return nil, err
	}
	var out []alert.Snapshot
	for uuid, st := range statuses {
		name := uuid
		if c, ok := clients[uuid]; ok {
			name = c.Name
		}
		snap := alert.Snapshot{UUID: uuid, Name: name, Online: st.Online, Ping: map[string]float64{}, Loss: map[string]float64{}}
		if t, err := time.Parse(time.RFC3339, st.Time); err == nil {
			snap.LastSeen = t
		}
		for _, p := range st.Ping {
			snap.Ping[p.Name] = p.Latest
			snap.Loss[p.Name] = p.Loss
		}
		out = append(out, snap)
	}
	return out, nil
}

// AdminDeleteProbe 删除探针任务。
func (s *Store) AdminDeleteProbe(client, name string) error {
	_, err := s.db.Exec(`DELETE FROM probe_tasks WHERE client=? AND name=?`, client, name)
	return err
}

// AdminProbeTaskNames 返回某 client 的探针名列表（Agent 配置下发用）。
func (s *Store) AdminProbeTaskNames(client string) ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM probe_tasks WHERE client=?`, client)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// AdminDeleteNode 删除节点及其历史。
func (s *Store) AdminDeleteNode(uuid string) error {
	if _, err := s.db.Exec(`DELETE FROM status_history WHERE client=?`, uuid); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM nodes WHERE uuid=?`, uuid)
	return err
}
