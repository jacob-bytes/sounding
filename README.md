# sounding

**兼容 Komari RPC 契约的自主分布式探针（Go）** —— 主控 + Agent 自托管实现。

- 主控（`sounding-server`）：接收 Agent 上报 + 存储 + 暴露 Komari 兼容 API
- Agent（`sounding-agent`）：节点端采集（CPU/内存/磁盘/网络…）+ 定时上报
- 前端：直接复用 [komari-theme-ink](https://github.com/jacob-bytes/komari-theme-ink)（契约对齐即接入）

## 状态

⚠️ 骨架阶段：契约清单（`contracts/`）为当前核心交付——主控/Agent 按契约实现。

## 构建（后续）

```bash
go build ./cmd/server
go build ./cmd/agent
```

## 致谢

- 前端契约参考：[komari-theme-ink](https://github.com/jacob-bytes/komari-theme-ink)
- 上游：[Komari Monitor](https://github.com/elevenhq/komari-monitor)
