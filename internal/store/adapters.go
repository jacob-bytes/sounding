package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jacob-bytes/sounding/internal/alert"
	"github.com/jacob-bytes/sounding/internal/api"
	"github.com/jacob-bytes/sounding/internal/collect"
	"github.com/jacob-bytes/sounding/internal/probe"
)

// offlineAfter 超过该时长无上报视为离线。
const offlineAfter = 60 * time.Second

// Nodes 返回 map[uuid]Client（ink 契约）。
func (s *Store) Nodes() (map[string]api.Client, error) {
	rows, err := s.db.Query(`SELECT uuid, name, cpu_name, arch, os, region, cpu_cores, mem_total, swap_total, disk_total, kernel_version, virtualization, price, billing_cycle, currency, expired_at, group_name, tags, public_remark FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]api.Client{}
	for rows.Next() {
		var c api.Client
		if err := rows.Scan(&c.UUID, &c.Name, &c.CPUName, &c.Arch, &c.OS, &c.Region, &c.CPUCores, &c.MemTotal, &c.SwapTotal, &c.DiskTotal, &c.KernelVersion, &c.Virtualization, &c.Price, &c.BillingCycle, &c.Currency, &c.ExpiredAt, &c.Group, &c.Tags, &c.PublicRemark); err != nil {
			return nil, err
		}
		c.Groups = []string{c.Group}
		out[c.UUID] = c
	}
	return out, rows.Err()
}

// LatestStatus 返回 map[uuid]NodeStatus（含 online/uptime/ping 汇总）。
func (s *Store) LatestStatus() (map[string]api.NodeStatus, error) {
	rows, err := s.db.Query(`SELECT client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, ram_total, swap_total, disk_total, temp, load5, load15, uptime
		FROM status_history WHERE rowid IN (SELECT MAX(rowid) FROM status_history GROUP BY client)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]api.NodeStatus{}
	for rows.Next() {
		var n api.NodeStatus
		if err := rows.Scan(&n.Client, &n.Time, &n.CPU, &n.RAM, &n.Swap, &n.Load, &n.Disk, &n.NetIn, &n.NetOut, &n.NetTotalUp, &n.NetTotalDown, &n.Process, &n.Connections, &n.ConnectionsUDP, &n.RAMTotal, &n.SwapTotal, &n.DiskTotal, &n.Temp, &n.Load5, &n.Load15, &n.Uptime); err != nil {
			return nil, err
		}
		out[n.Client] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 一次性聚合探针窗口（避免每节点 N+1 查询）
	clients := make([]string, 0, len(out))
	for uuid := range out {
		clients = append(clients, uuid)
	}
	pings, err := s.pingSummaries(clients)
	if err != nil {
		return nil, err
	}
	for uuid, n := range out {
		n.Online = isOnline(n.Time)
		if n.Uptime <= 0 {
			n.Uptime = s.firstSeenUptime(uuid)
		}
		if p, ok := pings[uuid]; ok {
			n.Ping = p
		}
		out[uuid] = n
	}
	return out, nil
}

// isOnline 根据最后上报时间判断在线。
func isOnline(lastTime string) bool {
	t, err := time.Parse(time.RFC3339, lastTime)
	if err != nil {
		return false
	}
	return time.Since(t) < offlineAfter
}

// firstSeenUptime 无 Agent uptime 时用首条上报时间近似（兜底）。
func (s *Store) firstSeenUptime(uuid string) float64 {
	var first string
	_ = s.db.QueryRow(`SELECT time FROM status_history WHERE client=? ORDER BY time ASC LIMIT 1`, uuid).Scan(&first)
	if ft, err := time.Parse(time.RFC3339, first); err == nil {
		return time.Since(ft).Seconds()
	}
	return 0
}

// pingWindow 每个探针任务参与统计的最近记录条数。
const pingWindow = 150

// pingSummaries 一次性聚合所有 client 的探针窗口。
// 丢包 = 窗口内 value<0 的比例；均值/极值/tail 仅统计成功样本（排除 -1）。
func (s *Store) pingSummaries(clients []string) (map[string]map[string]api.NodeStatusPing, error) {
	taskRows, err := s.db.Query(`SELECT id, client, name FROM probe_tasks WHERE enabled=1`)
	if err != nil {
		return nil, err
	}
	defer taskRows.Close()
	type task struct {
		id     int
		client string
		name   string
	}
	var tasks []task
	for taskRows.Next() {
		var t task
		if err := taskRows.Scan(&t.id, &t.client, &t.name); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	if err := taskRows.Err(); err != nil {
		return nil, err
	}

	// 每个 (client, task) 只取最近 pingWindow 条
	recRows, err := s.db.Query(`SELECT client, task_id, value FROM (
			SELECT client, task_id, value, ROW_NUMBER() OVER (PARTITION BY client, task_id ORDER BY time DESC, rowid DESC) AS rn
			FROM probe_records
		) WHERE rn <= ? ORDER BY client, task_id, rn`, pingWindow)
	if err != nil {
		return nil, err
	}
	defer recRows.Close()
	series := map[string][]float64{} // key: client|taskID，按时间倒序（[0] 最新）
	for recRows.Next() {
		var client string
		var taskID int
		var value float64
		if err := recRows.Scan(&client, &taskID, &value); err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s|%d", client, taskID)
		series[key] = append(series[key], value)
	}
	if err := recRows.Err(); err != nil {
		return nil, err
	}

	out := map[string]map[string]api.NodeStatusPing{}
	for _, client := range clients {
		for _, t := range tasks {
			if t.client != "*" && t.client != client {
				continue
			}
			if out[client] == nil {
				out[client] = map[string]api.NodeStatusPing{}
			}
			out[client][fmt.Sprint(t.id)] = summarizePing(t.name, series[fmt.Sprintf("%s|%d", client, t.id)])
		}
	}
	return out, nil
}

// summarizePing 计算单个任务在窗口内的汇总。
func summarizePing(name string, vals []float64) api.NodeStatusPing {
	p := api.NodeStatusPing{Name: name}
	if len(vals) == 0 {
		p.Latest, p.Loss = -1, 100
		return p
	}
	p.Latest = vals[0] // 最新一条（-1 表示丢包）
	ok := make([]float64, 0, len(vals))
	fail := 0
	for _, v := range vals {
		if v < 0 {
			fail++
			continue
		}
		ok = append(ok, v)
	}
	p.Loss = float64(fail) / float64(len(vals)) * 100
	if len(ok) == 0 {
		p.Latest, p.Loss = -1, 100
		return p
	}
	sum, minV, maxV := 0.0, ok[0], ok[0]
	for _, v := range ok {
		sum += v
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	p.Avg = sum / float64(len(ok))
	p.Min, p.Max = minV, maxV
	sorted := append([]float64(nil), ok...)
	sort.Float64s(sorted)
	p.Tail = sorted[int(float64(len(sorted)-1)*0.95)]
	return p
}

// RecentStatus 返回 {count, records}（ink 契约）。
func (s *Store) RecentStatus(client string, limit int) (map[string]any, error) {
	rows, err := s.db.Query(`SELECT client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, ram_total, swap_total, disk_total, temp, load5, load15
		FROM (SELECT * FROM status_history WHERE client = ? ORDER BY time DESC LIMIT ?) ORDER BY time ASC`, client, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var recs []api.StatusRecord
	for rows.Next() {
		var r api.StatusRecord
		if err := rows.Scan(&r.Client, &r.Time, &r.CPU, &r.RAM, &r.Swap, &r.Load, &r.Disk, &r.NetIn, &r.NetOut, &r.NetTotalUp, &r.NetTotalDown, &r.Process, &r.Connections, &r.ConnectionsUDP, &r.RAMTotal, &r.SwapTotal, &r.DiskTotal, &r.Temp, &r.Load5, &r.Load15); err != nil {
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

// ProbeClients 返回需要执行探针的 client 集合（节点 + 已有节点级任务）。
func (s *Store) ProbeClients() ([]string, error) {
	rows, err := s.db.Query(`SELECT client FROM probe_tasks WHERE client <> '*'
		UNION SELECT uuid FROM nodes ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		if c != "" {
			out = append(out, c)
		}
	}
	return out, rows.Err()
}

// UpsertProbeTask 按 (client, name) 创建/更新探针任务（Agent 上报路径）。
// 注意：冲突时保留已配置的 interval_sec——Agent 会把主控下发的任务回传，
// 若一并覆盖会把管理页设置的间隔重置为默认值。
func (s *Store) UpsertProbeTask(client, target, name, typ string, intervalSec float64, enabled bool) error {
	e := 0
	if enabled {
		e = 1
	}
	_, err := s.db.Exec(`INSERT INTO probe_tasks (client, target, type, name, interval_sec, enabled) VALUES (?,?,?,?,?,?)
		ON CONFLICT(client, name) DO UPDATE SET target=excluded.target, type=excluded.type, enabled=excluded.enabled`,
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
// 硬件信息（CPU/内存/磁盘/架构/系统/内核）随每次上报刷新；
// region/价格/分组等运营字段只在手动管理 API 修改，避免被上报覆盖。
func (s *Store) UpsertNode(uuid, name, info string) error {
	o := parseOSInfo(info)
	cpuName := orDefault(o.CPUName, "unknown")
	arch := orDefault(o.Arch, "unknown")
	osName := orDefault(o.OS, "sounding-agent")
	region := orDefault(o.Region, "UNKNOWN")
	cores := o.CPUCores
	if cores <= 0 {
		cores = 1
	}
	_, err := s.db.Exec(`INSERT INTO nodes (uuid, name, cpu_name, arch, os, region, cpu_cores, mem_total, swap_total, disk_total, kernel_version, virtualization, price, billing_cycle, currency, expired_at, group_name, tags, public_remark)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(uuid) DO UPDATE SET
		  name=excluded.name,
		  cpu_name=CASE WHEN excluded.cpu_name NOT IN ('','unknown') THEN excluded.cpu_name ELSE nodes.cpu_name END,
		  arch=CASE WHEN excluded.arch NOT IN ('','unknown') THEN excluded.arch ELSE nodes.arch END,
		  os=CASE WHEN excluded.os NOT IN ('','sounding-agent') THEN excluded.os ELSE nodes.os END,
		  cpu_cores=CASE WHEN excluded.cpu_cores > 0 THEN excluded.cpu_cores ELSE nodes.cpu_cores END,
		  mem_total=CASE WHEN excluded.mem_total > 0 THEN excluded.mem_total ELSE nodes.mem_total END,
		  swap_total=CASE WHEN excluded.swap_total > 0 THEN excluded.swap_total ELSE nodes.swap_total END,
		  disk_total=CASE WHEN excluded.disk_total > 0 THEN excluded.disk_total ELSE nodes.disk_total END,
		  kernel_version=CASE WHEN excluded.kernel_version <> '' THEN excluded.kernel_version ELSE nodes.kernel_version END,
		  virtualization=CASE WHEN excluded.virtualization <> '' THEN excluded.virtualization ELSE nodes.virtualization END`,
		uuid, name, cpuName, arch, osName, region, cores, o.MemTotal, o.SwapTotal, o.DiskTotal, o.KernelVersion, o.Virtualization,
		0, 0, "USD", "", "agent", "", "sounding-agent 节点")
	return err
}

// parseOSInfo 解析 Agent 上报的 os_info（兼容 snake_case / CamelCase 两种键名）。
func parseOSInfo(info string) collect.OSInfo {
	var o collect.OSInfo
	if info == "" {
		return o
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(info), &m); err != nil {
		return o
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if raw, ok := m[k]; ok {
				var s string
				if json.Unmarshal(raw, &s) == nil && s != "" {
					return s
				}
			}
		}
		return ""
	}
	num := func(keys ...string) float64 {
		for _, k := range keys {
			if raw, ok := m[k]; ok {
				var f float64
				if json.Unmarshal(raw, &f) == nil {
					return f
				}
			}
		}
		return 0
	}
	o.HostName = pick("host_name", "HostName")
	o.CPUName = pick("cpu_name", "CPUName")
	o.Arch = pick("arch", "Arch")
	o.OS = pick("os", "OS")
	o.Region = pick("region", "Region")
	o.KernelVersion = pick("kernel_version", "KernelVersion")
	o.Virtualization = pick("virtualization", "Virtualization")
	o.CPUCores = num("cpu_cores", "CPUCores")
	o.MemTotal = num("mem_total", "MemTotal")
	o.SwapTotal = num("swap_total", "SwapTotal")
	o.DiskTotal = num("disk_total", "DiskTotal")
	return o
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// InsertStatus 写入一条状态（client = uuid）。
func (s *Store) InsertStatus(uuid string, st api.AgentStatus) error {
	_, err := s.db.Exec(`INSERT INTO status_history (client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, temp, load5, load15, ram_total, swap_total, disk_total, uptime)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		uuid, time.Now().Format(time.RFC3339), st.CPU, st.RAM, st.Swap, st.Load, st.Disk, st.NetIn, st.NetOut,
		st.NetTotalUp, st.NetTotalDown, st.Process, st.Connections, st.ConnectionsUDP, st.Temp, st.Load5, st.Load15, st.RAMTotal, st.SwapTotal, st.DiskTotal, st.Uptime)
	return err
}

// AdminNodes 管理视角节点列表。
func (s *Store) AdminNodes() (map[string]api.Client, error) { return s.Nodes() }

// AdminUpsertNode 手动添加/更新节点。
func (s *Store) AdminUpsertNode(n api.Client) error {
	_, err := s.db.Exec(`INSERT INTO nodes (uuid, name, cpu_name, arch, os, region, cpu_cores, mem_total, swap_total, disk_total, price, billing_cycle, currency, expired_at, group_name, tags, public_remark)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(uuid) DO UPDATE SET name=excluded.name, region=excluded.region, price=excluded.price`,
		n.UUID, n.Name, n.CPUName, n.Arch, n.OS, n.Region, n.CPUCores, n.MemTotal, n.SwapTotal, n.DiskTotal, n.Price, n.BillingCycle, n.Currency, n.ExpiredAt, n.Group, n.Tags, n.PublicRemark)
	return err
}

// AdminProbeTasks 管理视角探针任务。
func (s *Store) AdminProbeTasks() ([]api.AdminProbe, error) {
	rows, err := s.db.Query(`SELECT id, client, target, name, type, interval_sec, enabled FROM probe_tasks ORDER BY client, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.AdminProbe
	for rows.Next() {
		var p api.AdminProbe
		var enabled int
		if err := rows.Scan(&p.ID, &p.Client, &p.Target, &p.Name, &p.Type, &p.IntervalSec, &enabled); err != nil {
			return nil, err
		}
		p.Enabled = enabled == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// AdminUpsertProbe 管理视角创建/更新探针（默认 60s）。
func (s *Store) AdminUpsertProbe(client, target, name, typ string, enabled bool) error {
	return s.AdminUpsertProbeInterval(client, target, name, typ, 60, enabled)
}

// AdminUpsertProbeInterval 创建/更新探针（可指定间隔秒）。
func (s *Store) AdminUpsertProbeInterval(client, target, name, typ string, intervalSec float64, enabled bool) error {
	if typ == "" {
		typ = "icmp"
	}
	if intervalSec <= 0 {
		intervalSec = 60
	}
	e := 0
	if enabled {
		e = 1
	}
	_, err := s.db.Exec(`INSERT INTO probe_tasks (client, target, name, type, interval_sec, enabled) VALUES (?,?,?,?,?,?)
		ON CONFLICT(client, name) DO UPDATE SET target=excluded.target, type=excluded.type, interval_sec=excluded.interval_sec, enabled=excluded.enabled`,
		client, target, name, typ, intervalSec, e)
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
