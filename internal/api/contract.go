package api

// Client ink 契约：getNodes 返回 map[uuid]Client。
type Client struct {
	UUID             string   `json:"uuid"`
	Name             string   `json:"name"`
	CPUName          string   `json:"cpu_name"`
	Virtualization   string   `json:"virtualization"`
	Arch             string   `json:"arch"`
	CPUCores         float64  `json:"cpu_cores"`
	OS               string   `json:"os"`
	KernelVersion    string   `json:"kernel_version"`
	Region           string   `json:"region"`
	PublicRemark     string   `json:"public_remark"`
	MemTotal         float64  `json:"mem_total"`
	SwapTotal        float64  `json:"swap_total"`
	DiskTotal        float64  `json:"disk_total"`
	Weight           float64  `json:"weight"`
	Price            float64  `json:"price"`
	BillingCycle     float64  `json:"billing_cycle"`
	AutoRenewal      bool     `json:"auto_renewal"`
	Currency         string   `json:"currency"`
	ExpiredAt        string   `json:"expired_at"`
	Group            string   `json:"group"`
	Groups           []string `json:"groups"`
	Tags             string   `json:"tags"`
	Hidden           bool     `json:"hidden"`
}

// NodeStatusPing 探针汇总（ink 契约）。
type NodeStatusPing struct {
	Name   string  `json:"name"`
	Latest float64 `json:"latest"`
	Avg    float64 `json:"avg"`
	Tail   float64 `json:"tail"`
	Loss   float64 `json:"loss"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

// NodeStatus ink 契约：getNodesLatestStatus 返回 map[uuid]NodeStatus。
type NodeStatus struct {
	Client         string                    `json:"client"`
	Time           string                    `json:"time"`
	CPU            float64                   `json:"cpu"`
	GPU            float64                   `json:"gpu"`
	RAM            float64                   `json:"ram"`
	RAMTotal       float64                   `json:"ram_total"`
	Swap           float64                   `json:"swap"`
	SwapTotal      float64                   `json:"swap_total"`
	Load           float64                   `json:"load"`
	Load5          float64                   `json:"load5"`
	Load15         float64                   `json:"load15"`
	Temp           float64                   `json:"temp"`
	Disk           float64                   `json:"disk"`
	DiskTotal      float64                   `json:"disk_total"`
	NetIn          float64                   `json:"net_in"`
	NetOut         float64                   `json:"net_out"`
	NetTotalUp     float64                   `json:"net_total_up"`
	NetTotalDown   float64                   `json:"net_total_down"`
	Process        float64                   `json:"process"`
	Connections    float64                   `json:"connections"`
	ConnectionsUDP float64                   `json:"connections_udp"`
	Online         bool                      `json:"online"`
	Uptime         float64                   `json:"uptime"`
	Ping           map[string]NodeStatusPing `json:"ping,omitempty"`
}
