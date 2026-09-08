<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import TrendChart from '@/components/TrendChart.vue'
import { CardX } from '@/components/ui/card-x'
import { rpc } from '@/services/rpc'
import type { Client, NodeStatus } from '@/services/rpc'
import { formatBytes, formatBytesSplit, formatSpeed, formatUptime, percent, statusOf } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const uuid = computed(() => String(route.params.uuid))

const node = ref<Client>()
const status = ref<NodeStatus>()
const records = ref<NodeStatus[]>([])
const loading = ref(true)

async function load() {
  loading.value = true
  const [nodes, statuses, recent] = await Promise.all([
    rpc.getNodes(),
    rpc.getNodesLatestStatus(),
    rpc.getNodeRecentStatus(uuid.value, 150),
  ])
  node.value = nodes[uuid.value]
  status.value = statuses[uuid.value]
  records.value = recent.records || []
  loading.value = false
}

onMounted(load)
watch(uuid, load)

const memPct = computed(() => percent(status.value?.ram ?? 0, status.value?.ram_total ?? 0))
const diskPct = computed(() => percent(status.value?.disk ?? 0, status.value?.disk_total ?? 0))
const swapPct = computed(() => percent(status.value?.swap ?? 0, status.value?.swap_total ?? 0))

/** 基本信息字段（ink 详情页风格） */
const infoFields = computed(() => {
  const n = node.value
  const s = status.value
  if (!n)
    return []
  const up = formatBytesSplit(s?.net_total_up ?? 0)
  const down = formatBytesSplit(s?.net_total_down ?? 0)
  return [
    { label: '系统', value: [n.os, n.arch].filter(Boolean).join(' · ') || '—' },
    { label: 'CPU', value: n.cpu_name || '—', tooltip: `${n.cpu_cores} vCPU` },
    { label: '内存 / 硬盘', value: `${formatBytes(s?.ram ?? 0)} / ${formatBytes(s?.disk ?? 0)}`, sub: `总量 ${formatBytes(s?.ram_total ?? 0)} / ${formatBytes(s?.disk_total ?? 0)}` },
    { label: '流量', value: '', upValue: `${up.value}${up.unit}`, downValue: `${down.value}${down.unit}` },
    { label: '运行时长', value: formatUptime(s?.uptime ?? 0) },
  ]
})

const pingList = computed(() => Object.values(status.value?.ping ?? {}))

function latencyTone(ms: number): string {
  if (ms < 0)
    return 'text-destructive'
  if (ms < 80)
    return 'text-success'
  if (ms < 150)
    return 'text-primary'
  if (ms < 220)
    return 'text-warning'
  return 'text-destructive'
}
</script>

<template>
  <div class="min-h-screen bg-background">
    <header class="sticky top-0 z-10 border-b border-border bg-background/80">
      <div class="mx-auto flex h-14 max-w-7xl items-center gap-3 px-4">
        <button class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-muted" @click="router.push('/')">
          <Icon icon="tabler:arrow-left" :width="14" /> 返回
        </button>
        <span class="truncate text-sm font-semibold text-foreground">{{ node?.name || '节点详情' }}</span>
        <span class="size-1.5 rounded-full" :class="status?.online ? 'bg-success' : 'bg-destructive'" />
        <span class="text-xs text-muted-foreground">{{ status?.online ? '在线' : '离线' }}</span>
        <span v-if="node?.region" class="ml-auto rounded-full bg-muted px-2 py-0.5 font-mono text-[10px] text-muted-foreground">{{ node.region }}</span>
      </div>
    </header>

    <main class="mx-auto max-w-7xl space-y-4 px-4 py-6">
      <!-- 基本信息 -->
      <div class="grid grid-cols-2 gap-x-6 gap-y-4 rounded-lg border border-border bg-card px-4 py-5 md:grid-cols-3 xl:grid-cols-5">
        <div v-for="(f, i) in infoFields" :key="f.label" class="min-w-0" :class="i < infoFields.length - 1 ? 'border-r border-border/60 pr-6' : ''">
          <div class="text-[11px] font-medium tracking-wider text-muted-foreground">{{ f.label }}</div>
          <div class="mt-1 truncate font-mono text-sm font-semibold text-foreground">
            <template v-if="f.upValue">
              <span>↑ {{ f.upValue }}</span><span class="text-muted-foreground"> · </span><span>↓ {{ f.downValue }}</span>
            </template>
            <template v-else>{{ f.value }}</template>
          </div>
          <div v-if="f.sub" class="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">{{ f.sub }}</div>
        </div>
      </div>

      <!-- 资源指标 -->
      <div class="grid grid-cols-2 gap-3 md:grid-cols-4">
        <CardX v-for="m in [
          { label: 'CPU', pct: status?.cpu ?? 0, detail: `${(status?.load ?? 0).toFixed(2)} / ${(status?.load5 ?? 0).toFixed(2)} / ${(status?.load15 ?? 0).toFixed(2)}` },
          { label: '内存', pct: memPct, detail: `${formatBytes(status?.ram ?? 0)} / ${formatBytes(status?.ram_total ?? 0)}` },
          { label: '硬盘', pct: diskPct, detail: `${formatBytes(status?.disk ?? 0)} / ${formatBytes(status?.disk_total ?? 0)}` },
          { label: 'Swap', pct: swapPct, detail: `${formatBytes(status?.swap ?? 0)} / ${formatBytes(status?.swap_total ?? 0)}` },
        ]" :key="m.label" size="small">
          <div class="p-3">
            <div class="flex items-baseline justify-between">
              <span class="text-xs text-muted-foreground">{{ m.label }}</span>
              <span class="font-mono text-sm font-semibold tabular-nums text-foreground">{{ m.pct.toFixed(1) }}%</span>
            </div>
            <div class="mt-1.5 h-1 overflow-hidden rounded-full bg-muted">
              <div class="h-full rounded-full" :class="{ default: 'bg-muted-foreground/40', warning: 'bg-warning', error: 'bg-destructive' }[statusOf(m.pct)]" :style="{ width: `${Math.min(m.pct, 100)}%` }" />
            </div>
            <div class="mt-1.5 font-mono text-[10px] text-muted-foreground">{{ m.detail }}</div>
          </div>
        </CardX>
      </div>

      <!-- 趋势图 -->
      <TrendChart :records="records" title="CPU / 内存 / 硬盘趋势" height="220px" />

      <!-- 网络 -->
      <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
        <CardX size="small">
          <div class="p-3">
            <div class="text-xs text-muted-foreground">实时速率</div>
            <div class="mt-1 flex items-center gap-3 font-mono text-lg font-semibold text-foreground">
              <span class="inline-flex items-center gap-1"><Icon icon="tabler:arrow-down" :width="14" class="text-muted-foreground" />{{ formatSpeed(status?.net_in ?? 0) }}</span>
              <span class="inline-flex items-center gap-1"><Icon icon="tabler:arrow-up" :width="14" class="text-muted-foreground" />{{ formatSpeed(status?.net_out ?? 0) }}</span>
            </div>
          </div>
        </CardX>
        <CardX size="small">
          <div class="p-3">
            <div class="text-xs text-muted-foreground">累计流量 / 温度 / 进程</div>
            <div class="mt-1 flex items-center gap-4 font-mono text-sm text-foreground">
              <span>↓ {{ formatBytes(status?.net_total_down ?? 0) }}</span>
              <span>↑ {{ formatBytes(status?.net_total_up ?? 0) }}</span>
              <span>{{ (status?.temp ?? 0).toFixed(1) }}°C</span>
              <span>{{ status?.process ?? 0 }} 进程</span>
            </div>
          </div>
        </CardX>
      </div>

      <!-- 探针延迟 -->
      <div v-if="pingList.length" class="rounded-lg border border-border bg-card">
        <div class="border-b border-border px-3 py-2 text-xs font-medium text-muted-foreground">探针延迟</div>
        <div class="grid grid-cols-2 gap-3 p-3 md:grid-cols-4">
          <div v-for="p in pingList" :key="p.name" class="rounded-md bg-muted/50 px-2.5 py-2">
            <div class="flex items-center justify-between">
              <span class="text-xs text-muted-foreground">{{ p.name }}</span>
              <span class="font-mono text-xs font-semibold tabular-nums" :class="latencyTone(p.latest)">{{ p.latest < 0 ? '超时' : `${p.latest.toFixed(0)}ms` }}</span>
            </div>
            <div class="mt-1 font-mono text-[10px] text-muted-foreground">
              最小 {{ p.min.toFixed(0) }} · 均 {{ p.avg.toFixed(0) }} · 最大 {{ p.max.toFixed(0) }} ms · 丢包 {{ p.loss.toFixed(1) }}%
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>
