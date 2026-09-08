package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RPCRequest 对应 ink rpc.ts 的 JSON-RPC 2.0 请求。
type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// RPCResponse 对应 JSON-RPC 2.0 响应。
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError JSON-RPC 2.0 错误对象。
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NodeRecord / StatusRecord 契约类型（对齐 contracts/contracts.md）。
type NodeRecord struct {
	UUID          string   `json:"uuid"`
	Name          string   `json:"name"`
	CPUName       string   `json:"cpu_name"`
	Virtualization string  `json:"virtualization"`
	Arch          string   `json:"arch"`
	CPUCores      float64  `json:"cpu_cores"`
	OS            string   `json:"os"`
	Region        string   `json:"region"`
	MemTotal      float64  `json:"mem_total"`
	SwapTotal     float64  `json:"swap_total"`
	DiskTotal     float64  `json:"disk_total"`
	Price         float64  `json:"price"`
	BillingCycle  float64  `json:"billing_cycle"`
	Currency      string   `json:"currency"`
	ExpiredAt     string   `json:"expired_at"`
	Group         string   `json:"group"`
	Groups        []string `json:"groups"`
	Tags          string   `json:"tags"`
	PublicRemark  string   `json:"public_remark"`
	Online        bool     `json:"online"`
	Uptime        float64  `json:"uptime"`
}

// StatusRecord 状态记录（字段与 ink StatusRecord 对齐）。
type StatusRecord struct {
	Client        string  `json:"client"`
	Time          string  `json:"time"`
	CPU           float64 `json:"cpu"`
	RAM           float64 `json:"ram"`
	Swap          float64 `json:"swap"`
	Load          float64 `json:"load"`
	Disk          float64 `json:"disk"`
	NetIn         float64 `json:"net_in"`
	NetOut        float64 `json:"net_out"`
	NetTotalUp    float64 `json:"net_total_up"`
	NetTotalDown  float64 `json:"net_total_down"`
	Process       float64 `json:"process"`
	Connections   float64 `json:"connections"`
	ConnectionsUDP float64 `json:"connections_udp"`
	RAMTotal      float64 `json:"ram_total"`
	SwapTotal     float64 `json:"swap_total"`
	DiskTotal     float64 `json:"disk_total"`
}

// Handler JSON-RPC 处理器（依赖注入 store）。
type Handler struct {
	store interface {
		Nodes() ([]NodeRecord, error)
		LatestStatus() ([]StatusRecord, error)
		RecentStatus(client string, limit int) ([]StatusRecord, error)
		PingRecords(client string, taskID int, limit int) ([]PingRecord, error)
	}
}

// NewHandler 构建处理器。
func NewHandler(s interface {
	Nodes() ([]NodeRecord, error)
	LatestStatus() ([]StatusRecord, error)
	RecentStatus(client string, limit int) ([]StatusRecord, error)
	PingRecords(client string, taskID int, limit int) ([]PingRecord, error)
}) *Handler {
	return &Handler{store: s}
}

// ServeHTTP 实现 JSON-RPC 2.0 POST。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req RPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, nil, -32700, "parse error")
		return
	}
	var result any
	var err error
	// ink 的 RPC 容器可能带命名空间前缀（如 common:getNodes / rpc.ping）——规范化
	method := req.Method
	if i := strings.LastIndex(method, ":"); i >= 0 {
		method = method[i+1:]
	} else if i := strings.LastIndex(method, "."); i >= 0 {
		method = method[i+1:]
	}
	switch method {
	case "ping":
		result = "pong"
	case "getMethods":
		result = []string{"getMethods", "getVersion", "getClient", "getHelp", "getNodes", "getNodesLatestStatus", "getNodeRecentStatus", "getPingRecords"}
	case "getVersion":
		result = map[string]string{"version": "0.1.0"}
	case "getClient":
		// 访客客户端信息（ink init 调用——宽松返回）
		result = map[string]any{"ip": "", "country": "CN", "country_code": "CN", "asn": "", "isp": ""}
	case "getHelp":
		result = "<html><body><h1>sounding</h1></body></html>"
	case "getNodes":
		result, err = h.store.Nodes()
	case "getNodesLatestStatus":
		result, err = h.store.LatestStatus()
	case "getPingRecords":
		var p struct {
			Client string `json:"client"`
			TaskID int    `json:"task_id"`
			Limit  int    `json:"limit"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if p.Limit <= 0 {
			p.Limit = 150
		}
		result, err = h.store.PingRecords(p.Client, p.TaskID, p.Limit)
	case "getNodeRecentStatus":
		var p struct {
			Client string `json:"client"`
			Limit  int    `json:"limit"`
		}
		if e := json.Unmarshal(req.Params, &p); e != nil {
			var arr []json.RawMessage
			_ = json.Unmarshal(req.Params, &arr)
			if len(arr) > 0 {
				_ = json.Unmarshal(arr[0], &p.Client)
			}
			if len(arr) > 1 {
				_ = json.Unmarshal(arr[1], &p.Limit)
			}
		}
		if p.Limit <= 0 {
			p.Limit = 150
		}
		result, err = h.store.RecentStatus(p.Client, p.Limit)
	default:
		writeErr(w, req.ID, -32601, "method not found: "+req.Method)
		return
	}
	if err != nil {
		writeErr(w, req.ID, -32000, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(RPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result})
}

func writeErr(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	_ = json.NewEncoder(w).Encode(RPCResponse{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: msg}})
}

// PingRecord 探针记录（契约）。
type PingRecord struct {
	Client string  `json:"client"`
	TaskID int     `json:"task_id"`
	Time   string  `json:"time"`
	Value  float64 `json:"value"`
}
