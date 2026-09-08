package store

import "github.com/jacob-bytes/sounding/internal/api"

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
