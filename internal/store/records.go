package store

import (
	"strings"
	"time"

	"github.com/jacob-bytes/sounding/internal/api"
)

// RecordsSince 返回指定时间窗口内的状态历史（client 为空表示全部节点，升序）。
func (s *Store) RecordsSince(client string, hours, limit int) ([]api.StatusRecord, error) {
	if hours <= 0 {
		hours = 1
	}
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339)
	q := `SELECT client, time, cpu, ram, swap, load, disk, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, ram_total, swap_total, disk_total, temp, load5, load15
		FROM status_history WHERE time >= ?`
	args := []any{cutoff}
	if client != "" {
		q += ` AND client = ?`
		args = append(args, client)
	}
	q += ` ORDER BY time ASC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]api.StatusRecord, 0, 128)
	for rows.Next() {
		var r api.StatusRecord
		if err := rows.Scan(&r.Client, &r.Time, &r.CPU, &r.RAM, &r.Swap, &r.Load, &r.Disk, &r.NetIn, &r.NetOut, &r.NetTotalUp, &r.NetTotalDown, &r.Process, &r.Connections, &r.ConnectionsUDP, &r.RAMTotal, &r.SwapTotal, &r.DiskTotal, &r.Temp, &r.Load5, &r.Load15); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ProbeRecordsSince 返回指定时间窗口内的探针记录（client/taskID 为 0/空表示不过滤，升序）。
func (s *Store) ProbeRecordsSince(client string, taskID, hours, limit int) ([]api.PingRecord, error) {
	if hours <= 0 {
		hours = 1
	}
	if limit <= 0 || limit > 20000 {
		limit = 5000
	}
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339)
	var b strings.Builder
	b.WriteString(`SELECT client, task_id, time, value FROM probe_records WHERE time >= ?`)
	args := []any{cutoff}
	if client != "" {
		b.WriteString(` AND client = ?`)
		args = append(args, client)
	}
	if taskID > 0 {
		b.WriteString(` AND task_id = ?`)
		args = append(args, taskID)
	}
	b.WriteString(` ORDER BY time ASC LIMIT ?`)
	args = append(args, limit)

	rows, err := s.db.Query(b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]api.PingRecord, 0, 256)
	for rows.Next() {
		var r api.PingRecord
		if err := rows.Scan(&r.Client, &r.TaskID, &r.Time, &r.Value); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
