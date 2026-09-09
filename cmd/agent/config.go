package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// FileConfig agent 配置文件（agent.yml）。
type FileConfig struct {
	Server   string   `yaml:"server"`    // 主控地址
	Token    string   `yaml:"token"`     // 上报 token
	NodeUUID string   `yaml:"node_uuid"` // 节点 ID（默认主机名）
	Interval string   `yaml:"interval"`  // 采集周期（如 15s）
	Probes   []string `yaml:"probes"`    // ["上海移动:223.5.5.5", ...]
}

// ConfigStore 带热重载的配置（SIGHUP + mtime 轮询）。
type ConfigStore struct {
	mu      sync.RWMutex
	path    string
	cfg     FileConfig
	lastMod time.Time
}

// LoadConfig 加载并启动热重载监控。
func LoadConfig(path string) *ConfigStore {
	cs := &ConfigStore{path: path}
	cs.reload()

	// SIGHUP 热重载
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGHUP)
		for range ch {
			log.Printf("config: SIGHUP 重载 %s", path)
			cs.reload()
		}
	}()
	// mtime 轮询兜底（跨平台）
	go func() {
		t := time.NewTicker(30 * time.Second)
		for range t.C {
			cs.reloadIfChanged()
		}
	}()
	return cs
}

func (cs *ConfigStore) reloadIfChanged() {
	st, err := os.Stat(cs.path)
	if err != nil {
		return
	}
	cs.mu.Lock()
	changed := st.ModTime().After(cs.lastMod)
	cs.mu.Unlock()
	if changed {
		cs.reload()
	}
}

func (cs *ConfigStore) reload() {
	data, err := os.ReadFile(cs.path)
	if err != nil {
		return // 文件不存在——继续用默认
	}
	var f FileConfig
	if err := yaml.Unmarshal(data, &f); err != nil {
		log.Printf("config: 解析 %s 失败: %v", cs.path, err)
		return
	}
	if st, err := os.Stat(cs.path); err == nil {
		cs.mu.Lock()
		cs.lastMod = st.ModTime()
		cs.mu.Unlock()
	}
	cs.mu.Lock()
	cs.cfg = f
	cs.mu.Unlock()
	log.Printf("config: 已加载 %s（server=%s, probes=%d）", cs.path, f.Server, len(f.Probes))
}

// Server 返回配置文件中的 server（空表示未配置，优先级由调用方决定）。
func (cs *ConfigStore) Server() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.cfg.Server
}

// Token 返回配置文件中的 token。
func (cs *ConfigStore) Token() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.cfg.Token
}

// NodeUUID 返回配置文件中的节点 uuid。
func (cs *ConfigStore) NodeUUID() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.cfg.NodeUUID
}

// Interval 返回配置文件中的采集周期（未配置/非法返回 0）。
func (cs *ConfigStore) Interval() time.Duration {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if cs.cfg.Interval != "" {
		if d, err := time.ParseDuration(cs.cfg.Interval); err == nil {
			return d
		}
	}
	return 0
}

// Probes 返回探针配置字符串（热重载后取最新）。
func (cs *ConfigStore) Probes() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return strings.Join(cs.cfg.Probes, ",")
}
