package store

import "github.com/jacob-bytes/sounding/internal/api"

func statusStub() api.AgentStatus {
	return api.AgentStatus{CPU: 12.5, RAM: 1 << 30, RAMTotal: 4 << 30, Load: 0.5, Temp: 45}
}
