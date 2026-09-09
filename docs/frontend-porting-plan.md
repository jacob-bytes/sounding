# 自研面板移植 ink 步骤

> 目标：把 `web/dashboard/` 逐阶段对齐 `komari-theme-ink` 的视觉与信息层级。
> 前置结论：**真实 ink 已可直接挂载使用**（见 [ink-parity.md](./ink-parity.md)）；本计划针对「无 Node 构建/内网兜底」的自研面板。
> 原则：只移植 **token、布局、信息层级与交互**，不 1:1 抄 26k 行；每阶段独立可交付、可回滚。

---

## 阶段总览

| 阶段 | 内容 | 产出文件 | 依赖 | 验收 |
|---|---|---|---|---|
| P0-0 | 基线与对比环境 | `docs/screenshots/*` | 无 | ink/自研截图可并排 |
| P0-1 | 设计 token 对齐 | `src/styles/main.css` | P0-0 | 亮暗色值与 ink 一致 |
| P0-2 | 应用外壳 | `AppHeader.vue` `AppFooter.vue` `App.vue` | P0-1 | 首页/详情共用外壳 |
| P0-3 | 首页结构 | `HomeOverviewCards.vue` `HomeToolbar.vue` `DashboardView.vue` | P0-2 | 概览/分组/搜索/视图切换 |
| P0-4 | 节点卡片 + 列表 | `NodeCard.vue` `NodeList.vue` | P0-1 | 信息层级对齐 |
| P0-5 | 详情页分区 + 图表 | `NodeDetailView.vue` `LoadChartCard.vue` `PingChartCard.vue` | P0-1 | 概览/负载/Ping 三 Tab |
| P1 | 交互与状态 | `useFavorites.ts` `PingMonitorDialog.vue` 等 | P0 | 收藏/筛选/弹窗/公告 |
| P2 | 高级工具 | 对比/导出/审计/背景等 | P1 | 按需 |

---

## P0-0 基线校准与对比环境

**先固定 ink 基线**（本地 / 远端 main / Release 三者可能漂移，动手前必做）：

```bash
# 1. 打印基线指标 + 本地 HEAD 与远端 main 是否一致（走 HTTPS，无需 SSH）
sh ../sounding/scripts/ink-inventory.sh ../komari-theme-ink

# 2. 用「官方 Release zip」而不是仓库里的 dist/（后者可能烘焙了 localhost:8100）
curl -fsSL -o /tmp/ink.zip https://github.com/jacob-bytes/komari-theme-ink/releases/download/<tag>/ink-build-<sha>.zip
unzip -q /tmp/ink.zip -d /tmp/ink-release
grep -o '"/api"' /tmp/ink-release/dist/assets/*.js | head -1   # 确认同源
```

1. 启动后端（演示数据 + 挂载两种面板）：
   ```bash
   # 终端 A：后端（挂官方 Release 产物）
   go run ./cmd/server -addr :8080 -db /tmp/dev.db -seed -static /tmp/ink-release/dist
   # 终端 B：自研面板 dev（Vite proxy 到 :8080）
   cd web/dashboard && bun install && bun run dev
   ```
2. 用 Playwright 分别截图（亮/暗、桌面 1440 / 移动 390）：
   - ink：`http://localhost:8080/`、`/node/<uuid>`
   - 自研：`http://localhost:5173/`、`/node/<uuid>`
3. 产物放 `docs/screenshots/{ink,self}-{home,detail}-{light,dark}.png`，作为每阶段回归基线。
4. 对照源码（与基线 commit 一致）：
   - token：`komari-theme-ink/src/styles/main.css`
   - 首页：`views/HomeView.vue`、`components/NodeGeneralCards.vue`、`NodeList.vue`
   - 卡片：`components/NodeCard.vue`
   - 详情：`views/InstanceDetail.vue`、`components/LoadChart.vue`、`PingChart.vue`

> 本文当前基线：远端 main `d128cf0` / v0.6.8（本地 HEAD 与其一致）；验证产物 `ink-build-d128cf0.zip` / v0.6.8。
> 若 `ink-inventory.sh` 输出与上面不一致，**先更新 [ink-parity.md](./ink-parity.md) 的基线表**，再开始移植。

**验收**：`docs/screenshots/` 下 8 张基线截图齐全；`ink-inventory.sh` 输出与文档基线一致。

---

## P0-1 设计 token 对齐

**改** `web/dashboard/src/styles/main.css`（Tailwind v4，用 `@theme` / CSS 变量）：

- 移植 ink 的 oklch 色板：`--background`、`--foreground`、`--card`、`--popover`、`--muted`、`--border`、`--input`、`--ring`、`--primary`、`--success`、`--destructive`、`--status-warn`。
- 亮色 `#f8fafc` slate 冷灰；暗色 `oklch(0.14 0.022 265)` 深蓝（非纯黑）。
- 三语义色收敛：蓝=主色/选中，绿=在线/健康，红=异常/丢包；延迟档位用琥珀 `--status-warn`。
- 全局数字字体：`.font-mono.tabular-nums`，表格/指标统一使用。
- 圆角/阴影：卡片 `rounded-lg border`，hover `-translate-y-0.5 shadow-md`。

**验收**：取色器对比亮/暗 `background/card/primary` 与 ink 一致；页面无旧 HSL 变量残留。

---

## P0-2 应用外壳

**新增**：

- `src/components/AppHeader.vue`
  - 左：站点名（`/api/public` 的 `sitename`）+ 描述。
  - 右：连接状态（WS/轮询）、数据新鲜度（最近数据到达时间）、主题切换、管理页入口。
- `src/components/AppFooter.vue`：版本号（`/api/version`）+ 站点描述。
- `src/composables/usePublicSettings.ts`：拉取 `/api/public` 并缓存。

**改** `src/App.vue`：`<AppHeader/> <RouterView/> <AppFooter/>`；移除各 view 里重复的 header。

**验收**：首页/详情共用同一外壳；Header 数据新鲜度随 WS 推送更新。

---

## P0-3 首页结构

**新增**：

- `src/components/HomeOverviewCards.vue`
  - 默认 4 张：在线节点 / 高负载 / 总流量 / 实时网速（合计上+下）。
  - 数据来自 `useRealtime()` 的 `nodes + statuses`，纯前端聚合，无需新接口。
  - 预留 `keys` prop，后续可对齐 ink 的可配置概览卡。
- `src/components/HomeToolbar.vue`
  - 分组 Tab（从 `node.groups` 聚合，含「全部」）。
  - 搜索框（名称/地区/IP/CPU，300ms 防抖）。
  - 卡片/列表视图切换（localStorage 记忆）。
  - 快捷筛选（P1 再接：收藏/离线/高负载/即将到期）。

**改** `src/views/DashboardView.vue`：
- 用 `HomeOverviewCards` 替换现有 4 张硬编码卡。
- 用 `HomeToolbar` 承载分组/搜索/视图切换。
- 离线节点置底；空状态带「清除筛选」。

**验收**：与 ink 首页布局一致；搜索/分组/视图切换可用且有记忆。

---

## P0-4 节点卡片 + 列表

**改** `src/components/NodeCard.vue`（按 ink 信息层级，从上到下）：

1. 头部：OS 图标 + 在线状态点（离线加脉冲/灰显）+ 名称 + 地区旗帜 + 收藏星。
2. 指标区：CPU / 内存 / 磁盘 / 流量 四项，各带细进度条 + 百分比 + 已用/总量。
3. 负载行：`load / load5 / load15`。
4. 网络区：上行/下行速率 + 累计上/下。
5. 探针区：延迟/丢包 **迷你点阵**（数据来自 `status.ping` 或 `getRecords(type=ping)`）。
6. 标签：`node.tags` 解析后的 pill。
7. 离线：整卡半透明 + 「离线 + 最后上报时间」遮罩。

**新增** `src/components/NodeList.vue`：表格式列表（名称/地区/CPU/内存/磁盘/网速/延迟），列可排序。

**验收**：与 ink 卡片逐项对照；长名称/多标签不溢出；移动端单列可用。

---

## P0-5 详情页分区 + 图表

**改** `src/views/NodeDetailView.vue`：

- 顶部：OS 图标 + 地区旗帜 + 收藏 + 上/下节点切换。
- 分区 Tab：`概览` / `负载` / `Ping`（本地记忆选中项）。
- 概览：资源指标卡（CPU/内存/磁盘/Swap）+ 设备信息（系统/CPU/架构/虚拟化/内核/运行时长/最后上报）。
- 负载：`LoadChartCard.vue`——多指标切换（CPU/内存/磁盘/网络/负载）、时间范围（1h/6h/24h）、数据来自 `rpc.getRecords(type=load)`。
- Ping：`PingChartCard.vue`——多任务延迟/丢包曲线 + P50/P99，数据来自 `rpc.getRecords(type=ping)` 或 `queryMetrics`。

**验收**：三个 Tab 均有数据；时间范围切换会重新拉取；与 ink 详情页结构一致。

---

## P1 交互与状态（P0 之后）

- `src/composables/useFavorites.ts`：收藏 uuid 集合（localStorage）。
- `HomeToolbar` 快捷筛选：收藏 / 离线 / 高负载（阈值可配）/ 即将到期。
- `src/components/PingMonitorDialog.vue`：点卡片探针区弹出多任务延迟/丢包详情。
- `src/components/SiteAlert.vue`：`/api/public` 的 `alertEnabled/alertTitle/alertContent`（Markdown 渲染）。
- 卡片尺寸档位：`mini/compact/comfortable/large`（localStorage）。

---

## P2 高级工具（按需）

对比面板、性价比、快照导出（CSV/JSON）、审计日志、访客信息、财务卡片、自定义背景、色觉友好模式、虚拟滚动、进场动效。

> 后端 `admin:getLogs`、`public:recordVisitorEvent` 目前是占位；做审计/访客前需先实现对应后端能力。

---

## 工程约定

- **目录/命名对齐 ink**：`components/ui/*`、`components/*Card.vue`、`composables/use*.ts`。
- **复用 helper**：`formatBytes/formatBytesPerSecond/formatUptime/percentile/statusOf` 放 `src/utils/`，全站统一。
- **不引新依赖**：优先用现有 `@iconify/vue`、`echarts`、`vue-echarts`；国旗可用 emoji 或 iconify。
- **每阶段独立提交**：`feat(dashboard): P0-x ...`，附前后截图。
- **回归验证**：
  ```bash
  cd web/dashboard && bun run type-check && bun run build
  # Playwright 截图对比 docs/screenshots 基线
  ```
- **后端不动**：ink 全量 RPC 契约已就绪（`internal/api/rpc_komari.go`），前端只需调用。

---

## 建议排期

| 阶段 | 预估 | 可交付 |
|---|---|---|
| P0-0 ~ P0-1 | 0.5 天 | 设计系统对齐 |
| P0-2 ~ P0-3 | 1 天 | 外壳 + 首页 |
| P0-4 | 1 天 | 卡片/列表 |
| P0-5 | 1 天 | 详情 + 图表 |
| P1 | 2 天 | 交互完善 |
| P2 | 按需 | 高级工具 |
