package api

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ==================== ink / Komari 兼容类型 ====================

// PingTaskInfo 探针任务摘要（ink common:getRecords type=ping / public:getPublicPingTasks）。
type PingTaskInfo struct {
	ID              int      `json:"id"`
	Weight          float64  `json:"weight,omitempty"`
	Name            string   `json:"name"`
	Interval        float64  `json:"interval"`
	Loss            float64  `json:"loss"`
	Type            string   `json:"type,omitempty"`
	Clients         []string `json:"clients,omitempty"`
	Min             float64  `json:"min"`
	Max             float64  `json:"max"`
	Avg             float64  `json:"avg"`
	Latest          float64  `json:"latest"`
	Total           int      `json:"total"`
	Valid           int      `json:"valid"`
	P50             float64  `json:"p50,omitempty"`
	P99             float64  `json:"p99,omitempty"`
	Stddev          float64  `json:"stddev,omitempty"`
	P99P50Ratio     float64  `json:"p99_p50_ratio,omitempty"`
	LossApproximate bool     `json:"loss_approximate,omitempty"`
}

// MetricDefinition 指标定义（ink public:listMetricDefinitions）。
type MetricDefinition struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Type          string            `json:"type"`
	Unit          string            `json:"unit,omitempty"`
	RetentionDays int               `json:"retention_days"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// MetricPoint 指标点。
type MetricPoint struct {
	Time  string         `json:"time"`
	Value *float64       `json:"value"`
	Count int            `json:"count,omitempty"`
	Tags  map[string]any `json:"tags,omitempty"`
}

// MetricSeries 指标序列。
type MetricSeries struct {
	MetricKey   string         `json:"metric_key"`
	EntityID    string         `json:"entity_id"`
	Type        string         `json:"type,omitempty"`
	Unit        string         `json:"unit,omitempty"`
	Tags        map[string]any `json:"tags,omitempty"`
	Downsampled bool           `json:"downsampled"`
	Count       int            `json:"count"`
	Points      []MetricPoint  `json:"points"`
}

// MetricQueryResponse（ink public:queryMetrics）。
type MetricQueryResponse struct {
	Start  string         `json:"start"`
	End    string         `json:"end"`
	Series []MetricSeries `json:"series"`
	Count  int            `json:"count"`
}

// PingMetricTaskStats 单任务 ping 统计（ink public:getPingMetricStats）。
type PingMetricTaskStats struct {
	EntityID        string         `json:"entity_id"`
	TaskID          string         `json:"task_id"`
	Name            string         `json:"name,omitempty"`
	Type            string         `json:"type,omitempty"`
	Interval        float64        `json:"interval,omitempty"`
	Tags            map[string]any `json:"tags"`
	Total           int            `json:"total"`
	Valid           int            `json:"valid"`
	Loss            float64        `json:"loss"`
	LossApproximate bool           `json:"loss_approximate"`
	Min             float64        `json:"min,omitempty"`
	Max             float64        `json:"max,omitempty"`
	Avg             float64        `json:"avg,omitempty"`
	Latest          float64        `json:"latest,omitempty"`
	P50             float64        `json:"p50,omitempty"`
	P99             float64        `json:"p99,omitempty"`
	Stddev          float64        `json:"stddev,omitempty"`
	P99P50Ratio     float64        `json:"p99_p50_ratio,omitempty"`
}

// PingMetricStatsResponse（ink public:getPingMetricStats）。
type PingMetricStatsResponse struct {
	Start           string                `json:"start"`
	End             string                `json:"end"`
	IntervalSeconds int                   `json:"interval_seconds"`
	Stats           []PingMetricTaskStats `json:"stats"`
	Count           int                   `json:"count"`
}

// ==================== 扩展 store 接口 ====================

// komariStore 为 ink 兼容方法所需的存储能力（store.Store 实现）。
type komariStore interface {
	RecordsSince(client string, hours, limit int) ([]StatusRecord, error)
	ProbeRecordsSince(client string, taskID, hours, limit int) ([]PingRecord, error)
	AdminProbeTasks() ([]AdminProbe, error)
	RecentStatus(client string, limit int) (map[string]any, error)
	Nodes() (map[string]Client, error)
}

// SetPublicSettings 注入公开设置（/public 同源）。
func (h *Handler) SetPublicSettings(f func() map[string]any) { h.publicSettings = f }

// SetVersion 注入服务版本。
func (h *Handler) SetVersion(v string) { h.version = v }

// ==================== 分发 ====================

// dispatchKomari 处理 ink/Komari 兼容方法（method 已去掉命名空间前缀）。
func (h *Handler) dispatchKomari(method string, req RPCRequest) (any, bool) {
	// 不依赖扩展 store 的方法
	switch method {
	case "getMethods":
		return []string{
			"getMethods", "getHelp", "getVersion", "getClient",
			"getNodes", "getNodesLatestStatus", "getNodeRecentStatus", "getRecords",
			"getPublicInfo", "getPublicSettings", "getBackendVersion",
			"getPublicNodesInformation", "getPublicClientRecentRecords",
			"getPublicRecordsByUUID", "getPublicPingRecords", "getPublicPingTasks",
			"listMetricDefinitions", "queryMetrics", "getPingMetricStats",
			"getPublicMe", "recordVisitorEvent", "getLogs", "editSettings",
		}, true
	case "getHelp":
		// ink 期望 MethodMeta[]；返回空数组比 HTML 字符串更安全
		return []any{}, true
	case "getVersion", "getBackendVersion":
		v := h.version
		if v == "" {
			v = "dev"
		}
		return map[string]any{"version": v}, true
	case "getPublicInfo", "getPublicSettings":
		return h.publicInfo(), true
	case "getPublicMe", "getMe":
		return map[string]any{"logged_in": false}, true
	case "recordVisitorEvent":
		return map[string]any{"status": "disabled"}, true
	case "getLogs":
		return map[string]any{"logs": []any{}, "total": 0}, true
	case "editSettings":
		return map[string]any{"status": "success"}, true
	case "listMetricDefinitions":
		return metricDefinitions(), true
	case "getNodeRecentStatus":
		return h.rpcNodeRecentStatus(req)
	}

	// 需要扩展 store 能力的方法
	ks, ok := h.store.(komariStore)
	if !ok {
		return nil, false
	}
	switch method {
	case "getRecords":
		return h.rpcGetRecords(ks, req)
	case "getPublicNodesInformation":
		nodes, err := ks.Nodes()
		if err != nil {
			return nil, false
		}
		out := make([]Client, 0, len(nodes))
		for _, n := range nodes {
			out = append(out, n)
		}
		return out, true
	case "getPublicClientRecentRecords":
		var p struct {
			UUID string `json:"uuid"`
		}
		_ = json.Unmarshal(req.Params, &p)
		res, err := ks.RecentStatus(p.UUID, 150)
		if err != nil {
			return nil, false
		}
		return res["records"], true
	case "getPublicRecordsByUUID":
		var p struct {
			UUID     string  `json:"uuid"`
			Hours    float64 `json:"hours"`
			LoadType string  `json:"load_type"`
		}
		_ = json.Unmarshal(req.Params, &p)
		records, err := ks.RecordsSince(p.UUID, intOr(p.Hours, 1), 5000)
		if err != nil {
			return nil, false
		}
		return map[string]any{"count": len(records), "records": records, "load_type": p.LoadType, "has_gpu_data": false}, true
	case "getPublicPingRecords":
		return h.rpcPingRecords(ks, req)
	case "getPublicPingTasks":
		return h.rpcPingTasks(ks)
	case "queryMetrics":
		return h.rpcQueryMetrics(ks, req)
	case "getPingMetricStats":
		return h.rpcPingMetricStats(ks, req)
	}
	return nil, false
}

func (h *Handler) publicInfo() map[string]any {
	if h.publicSettings != nil {
		return h.publicSettings()
	}
	return PublicSettings()
}

// rpcNodeRecentStatus 兼容 ink 的 {uuid, limit} 与旧版 {client, limit}。
func (h *Handler) rpcNodeRecentStatus(req RPCRequest) (any, bool) {
	var p struct {
		UUID   string `json:"uuid"`
		Client string `json:"client"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		// 兼容数组参数 [uuid, limit]
		var arr []json.RawMessage
		if json.Unmarshal(req.Params, &arr) == nil {
			if len(arr) > 0 {
				_ = json.Unmarshal(arr[0], &p.UUID)
			}
			if len(arr) > 1 {
				_ = json.Unmarshal(arr[1], &p.Limit)
			}
		}
	}
	client := p.UUID
	if client == "" {
		client = p.Client
	}
	if p.Limit <= 0 {
		p.Limit = 150
	}
	res, err := h.store.RecentStatus(client, p.Limit)
	if err != nil {
		return nil, false
	}
	return res, true
}

// rpcGetRecords common:getRecords（type=load|ping）。
func (h *Handler) rpcGetRecords(ks komariStore, req RPCRequest) (any, bool) {
	var p struct {
		Type          string  `json:"type"`
		UUID          string  `json:"uuid"`
		Client        string  `json:"client"`
		Hours         float64 `json:"hours"`
		TaskID        int     `json:"task_id"`
		LoadType      string  `json:"load_type"`
		MaxCount      int     `json:"maxCount"`
		MaxCountSnake int     `json:"max_count"`
	}
	_ = json.Unmarshal(req.Params, &p)
	uuid := p.UUID
	if uuid == "" {
		uuid = p.Client
	}
	hours := intOr(p.Hours, 1)
	limit := p.MaxCount
	if limit <= 0 {
		limit = p.MaxCountSnake
	}
	if limit <= 0 {
		limit = 1000
	}
	switch strings.ToLower(p.Type) {
	case "ping":
		records, err := ks.ProbeRecordsSince(uuid, p.TaskID, hours, limit)
		if err != nil {
			return nil, false
		}
		tasks, _ := buildPingTasks(ks, uuid, hours)
		return map[string]any{
			"count":      len(records),
			"records":    records,
			"tasks":      tasks,
			"basic_info": basicInfo(tasks),
		}, true
	default: // load
		records, err := ks.RecordsSince(uuid, hours, limit)
		if err != nil {
			return nil, false
		}
		if uuid != "" {
			return map[string]any{"records": records, "load_type": p.LoadType}, true
		}
		grouped := map[string][]StatusRecord{}
		for _, r := range records {
			grouped[r.Client] = append(grouped[r.Client], r)
		}
		return map[string]any{"records": grouped, "load_type": p.LoadType}, true
	}
}

// rpcPingRecords public:getPingRecords。
func (h *Handler) rpcPingRecords(ks komariStore, req RPCRequest) (any, bool) {
	var p struct {
		UUID     string  `json:"uuid"`
		TaskID   any     `json:"task_id"`
		Hours    float64 `json:"hours"`
		MaxCount int     `json:"max_count"`
	}
	_ = json.Unmarshal(req.Params, &p)
	taskID := anyToInt(p.TaskID)
	hours := intOr(p.Hours, 1)
	limit := p.MaxCount
	if limit <= 0 {
		limit = 5000
	}
	records, err := ks.ProbeRecordsSince(p.UUID, taskID, hours, limit)
	if err != nil {
		return nil, false
	}
	tasks, _ := buildPingTasks(ks, p.UUID, hours)
	return map[string]any{"count": len(records), "records": records, "tasks": tasks, "basic_info": basicInfo(tasks)}, true
}

// rpcPingTasks public:getPublicPingTasks。
func (h *Handler) rpcPingTasks(ks komariStore) (any, bool) {
	tasks, err := buildPingTasks(ks, "", 24)
	if err != nil {
		return nil, false
	}
	return tasks, true
}

// buildPingTasks 聚合每个探针任务的统计（client 为空表示全部节点）。
func buildPingTasks(ks komariStore, client string, hours int) ([]PingTaskInfo, error) {
	tasks, err := ks.AdminProbeTasks()
	if err != nil {
		return nil, err
	}
	records, err := ks.ProbeRecordsSince(client, 0, hours, 20000)
	if err != nil {
		return nil, err
	}
	byTask := map[int][]PingRecord{}
	clientsByTask := map[int]map[string]bool{}
	for _, r := range records {
		byTask[r.TaskID] = append(byTask[r.TaskID], r)
		if clientsByTask[r.TaskID] == nil {
			clientsByTask[r.TaskID] = map[string]bool{}
		}
		clientsByTask[r.TaskID][r.Client] = true
	}
	out := make([]PingTaskInfo, 0, len(tasks))
	for _, t := range tasks {
		if client != "" && t.Client != "*" && t.Client != client {
			continue
		}
		recs := byTask[t.ID]
		info := PingTaskInfo{ID: t.ID, Name: t.Name, Type: t.Type, Interval: t.IntervalSec, LossApproximate: true}
		vals := make([]float64, 0, len(recs))
		for _, r := range recs {
			info.Total++
			if r.Value >= 0 {
				info.Valid++
				vals = append(vals, r.Value)
			}
			info.Latest = r.Value
		}
		if info.Total > 0 {
			info.Loss = float64(info.Total-info.Valid) / float64(info.Total) * 100
		}
		if len(vals) > 0 {
			info.Min, info.Max, info.Avg, info.Stddev = stats(vals)
			info.P50 = percentile(vals, 50)
			info.P99 = percentile(vals, 99)
			if info.P50 > 0 {
				info.P99P50Ratio = info.P99 / info.P50
			}
		}
		if cs, ok := clientsByTask[t.ID]; ok {
			for c := range cs {
				info.Clients = append(info.Clients, c)
			}
			sort.Strings(info.Clients)
		}
		out = append(out, info)
	}
	return out, nil
}

func basicInfo(tasks []PingTaskInfo) []map[string]any {
	out := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, map[string]any{
			"client": t.Name, "loss": t.Loss, "min": t.Min, "max": t.Max,
		})
	}
	return out
}

// ==================== Metrics ====================

// metricDefinitions 返回 ink 负载图所需的指标定义。
func metricDefinitions() []MetricDefinition {
	defs := []struct{ name, desc, unit string }{
		{"cpu.usage", "CPU 使用率", "%"},
		{"load.average", "1 分钟负载", ""},
		{"memory.used", "内存已用", "bytes"},
		{"memory.total", "内存总量", "bytes"},
		{"swap.used", "Swap 已用", "bytes"},
		{"swap.total", "Swap 总量", "bytes"},
		{"temperature", "温度", "°C"},
		{"disk.used", "磁盘已用", "bytes"},
		{"disk.total", "磁盘总量", "bytes"},
		{"net.in.rate", "下行速率", "bytes/s"},
		{"net.out.rate", "上行速率", "bytes/s"},
		{"net.total.down", "累计下行", "bytes"},
		{"net.total.up", "累计上行", "bytes"},
		{"traffic.down", "流量下行", "bytes"},
		{"traffic.up", "流量上行", "bytes"},
		{"process.count", "进程数", ""},
		{"connections.tcp", "TCP 连接数", ""},
		{"connections.udp", "UDP 连接数", ""},
		{"ping.latency_ms", "探针延迟", "ms"},
		{"ping.loss", "探针丢包", "ratio"},
	}
	out := make([]MetricDefinition, 0, len(defs))
	for _, d := range defs {
		out = append(out, MetricDefinition{Name: d.name, Description: d.desc, Type: "gauge", Unit: d.unit, RetentionDays: 30})
	}
	return out
}

// statusMetricSelector 返回指标键对应的取值函数。
func statusMetricSelector(key string) func(StatusRecord) float64 {
	switch key {
	case "cpu.usage":
		return func(r StatusRecord) float64 { return r.CPU }
	case "load.average":
		return func(r StatusRecord) float64 { return r.Load }
	case "memory.used":
		return func(r StatusRecord) float64 { return r.RAM }
	case "memory.total":
		return func(r StatusRecord) float64 { return r.RAMTotal }
	case "swap.used":
		return func(r StatusRecord) float64 { return r.Swap }
	case "swap.total":
		return func(r StatusRecord) float64 { return r.SwapTotal }
	case "temperature":
		return func(r StatusRecord) float64 { return r.Temp }
	case "disk.used":
		return func(r StatusRecord) float64 { return r.Disk }
	case "disk.total":
		return func(r StatusRecord) float64 { return r.DiskTotal }
	case "net.in.rate":
		return func(r StatusRecord) float64 { return r.NetIn }
	case "net.out.rate":
		return func(r StatusRecord) float64 { return r.NetOut }
	case "net.total.down", "traffic.down":
		return func(r StatusRecord) float64 { return r.NetTotalDown }
	case "net.total.up", "traffic.up":
		return func(r StatusRecord) float64 { return r.NetTotalUp }
	case "process.count":
		return func(r StatusRecord) float64 { return r.Process }
	case "connections.tcp":
		return func(r StatusRecord) float64 { return r.Connections }
	case "connections.udp":
		return func(r StatusRecord) float64 { return r.ConnectionsUDP }
	}
	return nil
}

// rpcQueryMetrics public:queryMetrics（status_history + probe_records）。
func (h *Handler) rpcQueryMetrics(ks komariStore, req RPCRequest) (any, bool) {
	var p struct {
		MetricKeys []string `json:"metric_keys"`
		Metrics    []string `json:"metrics"`
		EntityID   string   `json:"entity_id"`
		UUID       string   `json:"uuid"`
		Hours      float64  `json:"hours"`
		MaxPoints  int      `json:"max_points"`
	}
	_ = json.Unmarshal(req.Params, &p)
	entity := p.EntityID
	if entity == "" {
		entity = p.UUID
	}
	hours := intOr(p.Hours, 1)
	limit := p.MaxPoints
	if limit <= 0 {
		limit = 500
	}
	keys := p.MetricKeys
	if len(keys) == 0 {
		keys = p.Metrics
	}

	records, err := ks.RecordsSince(entity, hours, limit)
	if err != nil {
		return nil, false
	}
	var pingRecords []PingRecord
	for _, k := range keys {
		if k == "ping.latency_ms" || k == "ping.loss" {
			pingRecords, _ = ks.ProbeRecordsSince(entity, 0, hours, limit*4)
			break
		}
	}
	now := time.Now()
	resp := MetricQueryResponse{
		Start: now.Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339),
		End:   now.Format(time.RFC3339),
	}
	for _, key := range keys {
		if sel := statusMetricSelector(key); sel != nil {
			series := MetricSeries{MetricKey: key, EntityID: entity, Downsampled: false}
			for _, r := range records {
				v := sel(r)
				vv := v
				series.Points = append(series.Points, MetricPoint{Time: r.Time, Value: &vv})
			}
			series.Count = len(series.Points)
			resp.Series = append(resp.Series, series)
		}
	}
	resp.Series = append(resp.Series, pingMetricSeries(entity, pingRecords, keys)...)
	resp.Count = len(resp.Series)
	return resp, true
}

// pingMetricSeries 生成 ping.latency_ms / ping.loss 序列（按 task 分组）。
func pingMetricSeries(entity string, records []PingRecord, keys []string) []MetricSeries {
	wantLatency, wantLoss := false, false
	for _, k := range keys {
		if k == "ping.latency_ms" {
			wantLatency = true
		}
		if k == "ping.loss" {
			wantLoss = true
		}
	}
	if !wantLatency && !wantLoss {
		return nil
	}
	byTask := map[int][]PingRecord{}
	for _, r := range records {
		byTask[r.TaskID] = append(byTask[r.TaskID], r)
	}
	var out []MetricSeries
	for taskID, recs := range byTask {
		tags := map[string]any{"task_id": taskID}
		if wantLatency {
			s := MetricSeries{MetricKey: "ping.latency_ms", EntityID: entity, Unit: "ms", Tags: tags}
			for _, r := range recs {
				if r.Value < 0 {
					continue
				}
				v := r.Value
				s.Points = append(s.Points, MetricPoint{Time: r.Time, Value: &v, Count: 1, Tags: tags})
			}
			s.Count = len(s.Points)
			out = append(out, s)
		}
		if wantLoss {
			s := MetricSeries{MetricKey: "ping.loss", EntityID: entity, Unit: "ratio", Tags: tags}
			for _, r := range recs {
				v := 0.0
				if r.Value < 0 {
					v = 1
				}
				s.Points = append(s.Points, MetricPoint{Time: r.Time, Value: &v, Count: 1, Tags: tags})
			}
			s.Count = len(s.Points)
			out = append(out, s)
		}
	}
	return out
}

// rpcPingMetricStats public:getPingMetricStats。
func (h *Handler) rpcPingMetricStats(ks komariStore, req RPCRequest) (any, bool) {
	var p struct {
		EntityID string  `json:"entity_id"`
		UUID     string  `json:"uuid"`
		TaskID   any     `json:"task_id"`
		TaskIDs  []any   `json:"task_ids"`
		Hours    float64 `json:"hours"`
	}
	_ = json.Unmarshal(req.Params, &p)
	entity := p.EntityID
	if entity == "" {
		entity = p.UUID
	}
	hours := intOr(p.Hours, 1)
	records, err := ks.ProbeRecordsSince(entity, 0, hours, 20000)
	if err != nil {
		return nil, false
	}
	tasks, _ := ks.AdminProbeTasks()
	nameByID := map[int]string{}
	typeByID := map[int]string{}
	for _, t := range tasks {
		nameByID[t.ID] = t.Name
		typeByID[t.ID] = t.Type
	}
	byTask := map[int][]PingRecord{}
	for _, r := range records {
		byTask[r.TaskID] = append(byTask[r.TaskID], r)
	}
	now := time.Now()
	resp := PingMetricStatsResponse{
		Start:           now.Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339),
		End:             now.Format(time.RFC3339),
		IntervalSeconds: 60,
	}
	for taskID, recs := range byTask {
		st := PingMetricTaskStats{
			EntityID: entity,
			TaskID:   itoa(taskID),
			Name:     nameByID[taskID],
			Type:     typeByID[taskID],
			Tags:     map[string]any{"task_id": taskID, "task_name": nameByID[taskID]},
			Total:    len(recs),
		}
		vals := make([]float64, 0, len(recs))
		for _, r := range recs {
			if r.Value >= 0 {
				st.Valid++
				vals = append(vals, r.Value)
			}
			st.Latest = r.Value
		}
		if st.Total > 0 {
			st.Loss = float64(st.Total-st.Valid) / float64(st.Total)
		}
		if len(vals) > 0 {
			st.Min, st.Max, st.Avg, st.Stddev = stats(vals)
			st.P50 = percentile(vals, 50)
			st.P99 = percentile(vals, 99)
			if st.P50 > 0 {
				st.P99P50Ratio = st.P99 / st.P50
			}
		}
		resp.Stats = append(resp.Stats, st)
	}
	resp.Count = len(resp.Stats)
	return resp, true
}

// ==================== 工具函数 ====================

func intOr(v float64, def int) int {
	if v <= 0 {
		return def
	}
	return int(v)
}

func anyToInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		if n, err := strconv.Atoi(t); err == nil {
			return n
		}
	}
	return 0
}

func itoa(v int) string { return strconv.Itoa(v) }

func stats(vals []float64) (minV, maxV, avg, stddev float64) {
	minV, maxV = vals[0], vals[0]
	sum := 0.0
	for _, v := range vals {
		sum += v
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	avg = sum / float64(len(vals))
	sq := 0.0
	for _, v := range vals {
		sq += (v - avg) * (v - avg)
	}
	stddev = math.Sqrt(sq / float64(len(vals)))
	return
}

func percentile(vals []float64, p float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
