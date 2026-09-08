# sounding · Komari RPC 契约（v1 锁定）

> 前端契约金标准：**komari-theme-ink** 的 `src/utils/rpc.ts` / `src/stores/nodes.ts`。
> 主控实现需**字段级对齐**——差异会导致 ink 仪表盘显示异常。

## 端点（RPC）

| 方法 | 说明 |
|---|---|
| `getMethods` | 列出可用 RPC 方法 |
| `getVersion` | 服务版本 |
| `getHelp` | 帮助页（HTML） |
| `getNodes` | 节点列表（NodeData[]） |
| `getNodesLatestStatus` | 全节点最新状态（StatusRecord[]） |
| `getNodeRecentStatus` | 单节点近期历史（records[]，≤150 条，按 time 升序） |
| `getPingRecords` | Ping 探测记录（PingRecord[]） |
| `getClient` | 客户端指纹/访客信息 |

## NodeData（getNodes）

```go
type NodeData struct {
    UUID             string  `json:"uuid"`
    Name             string  `json:"name"`
    CPUName          string  `json:"cpu_name"`
    Virtualization   string  `json:"virtualization"`
    Arch             string  `json:"arch"`
    CPUCores         float64 `json:"cpu_cores"`
    CPUPhysicalCores *float64 `json:"cpu_physical_cores,omitempty"`
    OS               string  `json:"os"`
    KernelVersion    string  `json:"kernel_version"`
    GPUNames         *string `json:"gpu_name,omitempty"`
    IPv4             *string `json:"ipv4,omitempty"`
    IPv6             *string `json:"ipv6,omitempty"`
    Region           string  `json:"region"`
    Remark           *string `json:"remark,omitempty"`
    PublicRemark     string  `json:"public_remark"`
    MemTotal         float64 `json:"mem_total"`
    SwapTotal        float64 `json:"swap_total"`
    DiskTotal        float64 `json:"disk_total"`
    Version          *string `json:"version,omitempty"`
    Weight           float64 `json:"weight"`
    Price            float64 `json:"price"`
    BillingCycle     float64 `json:"billing_cycle"`
    AutoRenewal      bool    `json:"auto_renewal"`
    Currency         string  `json:"currency"`
    ExpiredAt        string  `json:"expired_at"`
    Group            string  `json:"group"`
    Groups           []string `json:"groups"`
    Tags             string  `json:"tags"`
    Hidden           bool    `json:"hidden"`
    // ...（status: online/uptime/net_* 等字段以 ink NodeData 全文为准）
}
```

## StatusRecord（getNodesLatestStatus / getNodeRecentStatus）

```go
type StatusRecord struct {
    Client        string  `json:"client"`           // = NodeData.UUID
    Time          string  `json:"time"`             // ISO 8601
    CPU           float64 `json:"cpu"`
    GPU           float64 `json:"gpu"`
    GPUDetailed   []GPUDetails `json:"gpu_detailed_info,omitempty"`
    RAM           float64 `json:"ram"`
    RAMTotal      float64 `json:"ram_total"`
    Swap          float64 `json:"swap"`
    SwapTotal     float64 `json:"swap_total"`
    Load          float64 `json:"load"`
    Load5         float64 `json:"load5"`
    Load15        float64 `json:"load15"`
    Temp          float64 `json:"temp"`
    Disk          float64 `json:"disk"`
    DiskTotal     float64 `json:"disk_total"`
    NetIn         float64 `json:"net_in"`
    NetOut        float64 `json:"net_out"`
    NetTotalUp    float64 `json:"net_total_up"`
    NetTotalDown  float64 `json:"net_total_down"`
    Process       float64 `json:"process"`
    Connections   float64 `json:"connections"`
    ConnectionsUDP float64 `json:"connections_udp"`
}
```

## PingRecord（getPingRecords）

```go
type PingRecord struct {
    Client string  `json:"client"`
    TaskID int     `json:"task_id"`
    Time   string  `json:"time"`
    Value  float64 `json:"value"`
}
```

## 关键语义（实现注意）
| 项 | 约定 |
|---|---|
| `time` | ISO 8601（ink 用 dayjs 解析） |
| 单位 | bytes（ram/disk）· bytes/s（net_*）· 百分比（cpu/ram/disk 用量为**已用值**） |
| `client` | 节点 UUID（注册时分配） |
| `ram`/`swap` 等 | **已用值**（非总量——总量在 Total 字段） |
| 历史 | 按时间升序、≤150 条（前端滚动窗口） |
| 刷新语义 | 前端按 `dataUpdateInterval` 轮询——主控需保证 `getNodesLatestStatus` 时效 |
