# sounding 自研面板 vs ink 主题：差异分析与复刻路线

> 结论先行：**ink 是 26k+ 行的完整产品，sounding 自研面板是 ~1.1k 行的轻量降级方案**。
> 想「尽量复刻」有两条路，建议同时走：
>
> 1. **直接挂载真实 ink 产物**（推荐）——sounding 已实现 ink 所需的 RPC 契约，用官方 Release zip 实测零改动即可跑通首页/详情/图表；`scripts/install.sh deploy` 会自动下载安装。
> 2. **自研面板增量对齐**——按本文 P0→P2 优先级补齐视觉与交互，作为无 ink 产物时的兜底。

## 0. 基线（重要，先校准再动手）

本文所有结论基于以下**明确固定**的基线，避免「本地开发版 / 官方 Release / 远端 main」三者漂移：

| 项 | 值 |
|---|---|
| ink 源码仓库 | `../komari-theme-ink`（[GitHub](https://github.com/jacob-bytes/komari-theme-ink)） |
| 源码 commit / 版本 | `d128cf0` / `v0.6.8`（2026-09-10） |
| 验证用 Release 产物 | `ink-build-30c4eff.zip`（`v0.6.7`，`dist` 内置 `VITE_API_BASE=/api`） |
| 实测结果 | sounding-server 挂载该产物：首页 2 卡片、详情 7 图表 canvas、**0 控制台错误** |

> ⚠️ **不要用 ink 仓库里的 `dist/` 直接测试**：本地开发构建可能把 `VITE_API_BASE` 烘焙成 `http://localhost:8100`（我第一次就踩了这个坑），需要替换或重新构建。**官方 Release zip 是 `/api` 同源，零配置可用。**
>
> 远端可能领先本地，动手前先校准：
> ```bash
> cd ../komari-theme-ink && git fetch origin && git log --oneline -1 origin/main
> sh ../sounding/scripts/ink-inventory.sh ../komari-theme-ink   # 打印下方全部基线指标
> ```

---

## 1. 量化对比

| 维度 | ink 主题 | sounding `web/dashboard` |
|---|---|---|
| 源码规模 | 26,033 行（57 个 `.vue` + 79 个 `.ts`） | ≈ 1,100 行 |
| 视图 | `HomeView` + `InstanceDetail` + `NotFoundView` | `DashboardView` + `NodeDetailView` |
| 组件数 | 53 个组件（含 30 个 UI 原子组件） | 8 |
| 主题配置项 | 49 项 / 42 个 key（`komari-theme.json`） | 0（硬编码） |
| 状态管理 | Pinia（`stores/app.ts` 1.2k 行 + `stores/nodes.ts`） | 2 个 composable |
| 图表 | LoadChart 1.8k 行 + PingChart 958 行 + 虚拟滚动 | 1 个 ECharts 折线图 |
| 后端调用 | JSON-RPC + REST + WS + metrics 体系 | 4 个 RPC 方法 |
| 设计系统 | shadcn/reka-ui + oklch token + 全站等宽数字 | 手写 Tailwind + CSS 变量 |

## 2. 设计系统差异（最直观）

| 项 | ink | sounding 现状 | 建议 |
|---|---|---|---|
| 色板 | oklch：亮 `#f8fafc` slate / 暗 `oklch(0.14 0.022 265)` 深蓝（非纯黑） | HSL 变量，偏中性灰 | 直接移植 ink `styles/main.css` 的 `:root` / `.dark` token |
| 语义色 | 蓝 primary / 绿 success / 红 danger 三色收敛 | primary/success/warning/destructive 四色 | 去掉 warning 或改为 ink 的琥珀仅用于延迟档位 |
| 数字字体 | 全站 `font-mono tabular-nums` | 部分使用 | 全局应用 |
| 圆角/阴影 | `rounded-lg` + 细边框 + hover 阴影上浮 | 基本一致 | 补 hover `-translate-y-0.5` + `shadow-md` |
| 网格/密度 | 卡片网格虚拟滚动、mini/compact/comfortable/large 四档 | 固定三列 | 先加卡片尺寸档位（本地 localStorage） |
| 动效 | 进场 stagger、TransitionGroup、`disablePageAnimation` 开关 | 无 | P2 |
| 无障碍 | 焦点环、状态色+形状双表达、色觉友好模式 | 部分 aria | P2 |

## 3. 首页差异

| 能力 | ink | sounding | 优先级 |
|---|---|---|---|
| 站点公告（Markdown） | `alertEnabled/Title/Content` | 无 | P1 |
| 概览卡片 | 可配置 keys：在线/高负载/总流量/网速/内存/磁盘/剩余价值… | 固定 4 张（在线/上下行/累计） | **P0** |
| 分组 Tab | 按 `groups` 分组 | 无 | **P0** |
| 搜索 | 名称/地区/IP/CPU，300ms 防抖，空状态可清除 | 无 | **P0** |
| 快捷筛选 | 收藏/总流量/上行/下行/峰值/离线/高负载/即将到期，带计数 | 无 | P1 |
| 卡片/列表视图切换 | 二段式分段控件 + 本地记忆 | 无 | **P0** |
| 离线节点置底 | `offlineNodesLast` | 无 | P1 |
| 收藏 | `favorite`（localStorage） | 无 | P1 |
| 虚拟滚动 | 节点多时按行虚拟化 | 无 | P2 |
| 高级工具 | 对比 / 性价比 / 快照导出 / 审计日志 | 无 | P2 |
| 延迟监控弹窗 | `PingMonitorDialog` | 无 | P1 |
| 连接状态/数据新鲜度 | Header 绑定真实数据到达时间 + WS 状态 | 有简化版 | 保留 |
| 访客信息 | `VisitorInfo` + 审计 | 无 | P2 |

## 4. 节点卡片（NodeCard）差异

ink 卡片包含：OS 图标 + 地区旗帜、在线脉冲点、收藏星、离线半透明遮罩、CPU/内存/磁盘/流量四指标 + 进度条、负载三元组、上下行速率、累计流量、价格/剩余天数、三网延迟分项、**延迟/丢包点阵迷你柱状图**（悬浮查看单点时间+数值）、自定义标签、卡片尺寸档位。

sounding 现状：名称 + 地区、CPU/内存/磁盘进度、网络速率、运行时长、探针延迟 pill。

**P0/P1 建议**：OS 图标 + 旗帜、离线遮罩、延迟/丢包点阵、上下行 + 累计、标签；P2 再补收藏/价格/三网分项/尺寸档位。

## 5. 节点详情差异

| 区块 | ink | sounding | 优先级 |
|---|---|---|---|
| 顶部 | OS 图标 + 地区旗帜 + 收藏 + 上/下节点切换 + 厂商信息 | 名称 + 状态 + 地区 | P1 |
| 分区 Tab | 概览 / 负载 / Ping | 无（单页平铺） | **P0** |
| 资源指标 | CPU/内存/磁盘/流量/GPU 卡片 + 细进度条 | 4 张卡片 | 基本对齐 |
| 设备信息 | 系统/CPU/架构/虚拟化/内核/运行时长/最后上报，带 tooltip | 5 项 | P1 |
| 负载图 | LoadChart：多指标切换、时间范围、CSV、GPU | 单折线图 | **P0** |
| Ping 图 | PingChart：多任务、延迟/丢包、P50/P99 | 仅最新值列表 | P1 |
| 运行时间线 | NodeUptimeTimeline（在线/离线分段） | 无 | P1 |
| 财务卡片 | 价格/月成本/剩余时间/剩余价值/流量配额 | 无 | P2 |

## 6. 后端契约差异（已修复项）

ink 实际调用（`src/utils/rpc.ts`）与 sounding 实现对照：

| ink 调用 | 方法 | sounding 状态 |
|---|---|---|
| `getNodes` | `common:getNodes` | ✅ |
| `getNodesLatestStatus` | `common:getNodesLatestStatus` | ✅ |
| `getNodeRecentStatus(uuid)` | `common:getNodeRecentStatus` | ✅ 已支持 `{uuid}`（此前只认 `client`） |
| `getLoadRecords` | `common:getRecords {type:load}` | ✅ 新增 |
| `getPingRecords` | `common:getRecords {type:ping}` | ✅ 新增（含 `tasks`/`basic_info`） |
| `getPublicSettings` | `public:getPublicSettings` / REST `/api/public` | ✅ 返回完整 49 项 theme_settings |
| `getPublicPingTasks` | `public:getPublicPingTasks` | ✅ 新增 |
| `listMetricDefinitions` | `public:listMetricDefinitions` | ✅ 新增（20 个指标） |
| `queryMetrics` | `public:queryMetrics` | ✅ 新增（status_history + ping） |
| `getPingMetricStats` | `public:getPingMetricStats` | ✅ 新增 |
| `getPublicNodesInformation` / `getClientRecentRecords` / `getRecordsByUUID` / `getPublicPingRecords` / `getPublicMe` | `public:*` | ✅ 兼容别名 |
| `getVersion` / `getBackendVersion` | `rpc.getVersion` | ✅；REST `/api/version` 改为 `{status,data:{version,hash}}` |
| `getAuditLogs` / `editSettings` | `admin:*` | ⚠️ 占位（返回空日志 / success），审计功能未实现 |
| `recordVisitorEvent` | `public:recordVisitorEvent` | ⚠️ 返回 `disabled` |
| `getClient` | `rpc.getClient` | ✅（空指纹） |

**实测**（官方 Release 产物 `ink-build-30c4eff.zip` / v0.6.7）：`sounding-server -static <release-dist>` + 演示数据，Playwright 加载首页 **2 张节点卡、0 控制台错误**；点击进入详情页 **设备信息 + 7 个图表 canvas、0 错误**。

> 注意：只有 **本地开发构建** 的 `dist/` 可能烘焙 `VITE_API_BASE=http://localhost:8100`；官方 Release zip 与 `install.sh` 下载的产物都是同源 `/api`，无需修改。若自建，务必 `VITE_API_BASE=/api bun run build`。

## 7. 复刻路线（建议）

**P0（1–2 天，视觉/结构对齐，收益最大）**
1. 移植 ink oklch 设计 token + 全站等宽数字。
2. 首页：概览卡片（可配置 keys）、分组 Tab、搜索、卡片/列表切换、离线置底。
3. NodeCard：OS 图标/旗帜、离线遮罩、延迟/丢包点阵。
4. 详情：概览/负载/Ping 分区 Tab + 负载多指标图。

**P1（3–5 天）**
5. 快捷筛选（收藏/离线/高负载/即将到期）+ 收藏本地记忆。
6. 详情设备信息完整化 + PingChart + 运行时间线。
7. 公告、Header 数据新鲜度/WS 状态、延迟监控弹窗。

**P2（按需）**
8. 虚拟滚动、卡片尺寸档位、动效、色觉友好。
9. 对比/性价比/快照导出/审计日志/访客信息/财务卡片/自定义背景。
10. 若长期以 ink 为主 UI，可把自研面板定位为「无 JS 构建/内网兜底」，仅保留 P0。

## 8. 复刻时的取舍

- **不要 1:1 抄 26k 行**：ink 的价值在设计与信息架构，不在实现。优先对齐 token、布局、卡片信息层级。
- **后端契约优先**：契约对齐后可直接用 ink 官方 Release，比自研更快拿到完整功能。
- **自研面板定位**：轻量、无 Node 构建、Go embed 可单二进制交付；适合内网/演示/兜底。
