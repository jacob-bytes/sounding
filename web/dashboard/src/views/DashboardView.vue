<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { computed } from 'vue'
import NodeCard from '@/components/NodeCard.vue'
import { CardX } from '@/components/ui/card-x'
import { useRealtime } from '@/composables/useRealtime'
import { formatBytes, formatSpeed } from '@/utils/format'

const { nodes, statuses, connected, updatedAt } = useRealtime()

const list = computed(() =>
  Object.entries(nodes.value).map(([uuid, node]) => ({ uuid, node, status: statuses.value[uuid] })),
)

const online = computed(() => list.value.filter(i => i.status?.online).length)
const totalDown = computed(() => list.value.reduce((s, i) => s + (i.status?.net_in ?? 0), 0))
const totalUp = computed(() => list.value.reduce((s, i) => s + (i.status?.net_out ?? 0), 0))
const offline = computed(() => list.value.filter(i => i.status && !i.status.online).length)

const lastUpdated = computed(() => {
  if (!updatedAt.value)
    return '—'
  const diff = Math.max(0, Math.round((Date.now() - new Date(updatedAt.value).getTime()) / 1000))
  return diff < 2 ? '实时' : `${diff}s 前`
})
</script>

<template>
  <div class="min-h-screen bg-background">
    <header class="sticky top-0 z-10 border-b border-border bg-background/80">
      <div class="mx-auto flex h-14 max-w-7xl items-center justify-between px-4">
        <div class="flex items-center gap-2">
          <span class="text-base font-semibold tracking-tight text-foreground">sounding</span>
          <span class="text-xs text-muted-foreground">分布式探针</span>
        </div>
        <div class="flex items-center gap-3 text-xs text-muted-foreground">
          <span class="inline-flex items-center gap-1.5">
            <span class="size-1.5 rounded-full" :class="connected ? 'bg-success' : 'bg-muted-foreground/40'" />
            {{ connected ? '实时' : '轮询' }}
          </span>
          <span>{{ lastUpdated }}</span>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-7xl space-y-4 px-4 py-6">
      <!-- 概览 -->
      <div class="grid grid-cols-2 gap-3 md:grid-cols-4">
        <CardX size="small">
          <div class="p-4">
            <div class="text-xs text-muted-foreground">在线节点</div>
            <div class="mt-1 font-mono text-2xl font-semibold tabular-nums text-foreground">
              {{ online }} <span class="text-sm text-muted-foreground">/ {{ list.length }}</span>
            </div>
            <div class="mt-1 text-[11px] text-muted-foreground">
              {{ offline > 0 ? `● ${offline} 台离线` : '● 全部在线' }}
            </div>
          </div>
        </CardX>
        <CardX size="small">
          <div class="p-4">
            <div class="text-xs text-muted-foreground">下行速率</div>
            <div class="mt-1 font-mono text-2xl font-semibold tabular-nums text-foreground">{{ formatSpeed(totalDown) }}</div>
            <div class="mt-1 text-[11px] text-muted-foreground">全域合计</div>
          </div>
        </CardX>
        <CardX size="small">
          <div class="p-4">
            <div class="text-xs text-muted-foreground">上行速率</div>
            <div class="mt-1 font-mono text-2xl font-semibold tabular-nums text-foreground">{{ formatSpeed(totalUp) }}</div>
            <div class="mt-1 text-[11px] text-muted-foreground">全域合计</div>
          </div>
        </CardX>
        <CardX size="small">
          <div class="p-4">
            <div class="text-xs text-muted-foreground">累计流量</div>
            <div class="mt-1 font-mono text-2xl font-semibold tabular-nums text-foreground">
              {{ formatBytes(list.reduce((s, i) => s + (i.status?.net_total_down ?? 0), 0)) }}
            </div>
            <div class="mt-1 text-[11px] text-muted-foreground">下行总量</div>
          </div>
        </CardX>
      </div>

      <!-- 节点列表 -->
      <div v-if="list.length === 0" class="flex flex-col items-center justify-center gap-2 rounded-lg border border-border py-20 text-muted-foreground">
        <Icon icon="tabler:server-off" :width="28" />
        <span class="text-sm">暂无节点</span>
        <span class="text-xs">运行 sounding-agent 或访问 /admin/ 添加</span>
      </div>
      <div v-else class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <NodeCard v-for="item in list" :key="item.uuid" :node="item.node" :status="item.status" />
      </div>
    </main>
  </div>
</template>
