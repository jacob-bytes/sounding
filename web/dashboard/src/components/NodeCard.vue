<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { Client, NodeStatus } from '@/services/rpc'
import { formatBytes, formatSpeed, formatUptime, latencyClass, percent, statusOf } from '@/utils/format'

const props = defineProps<{ node: Client, status?: NodeStatus }>()
const router = useRouter()

const st = computed(() => props.status)
const cpuPct = computed(() => st.value?.cpu ?? 0)
const memPct = computed(() => percent(st.value?.ram ?? 0, st.value?.ram_total ?? 0))
const diskPct = computed(() => percent(st.value?.disk ?? 0, st.value?.disk_total ?? 0))
const pingList = computed(() => Object.values(st.value?.ping ?? {}))
</script>

<template>
  <div
    class="cursor-pointer rounded-lg border border-border bg-card transition-colors hover:border-primary/30"
    role="button" tabindex="0"
    @click="router.push(`/node/${node.uuid}`)"
    @keydown.enter="router.push(`/node/${node.uuid}`)"
  >
    <div class="flex items-center gap-2 border-b border-border px-3 py-2.5">
      <span class="size-1.5 shrink-0 rounded-full" :class="st?.online ? 'bg-success' : 'bg-destructive'" />
      <span class="truncate text-sm font-medium text-foreground">{{ node.name }}</span>
      <span
        v-if="node.region"
        class="ml-auto shrink-0 rounded-full bg-muted px-2 py-0.5 font-mono text-[10px] text-muted-foreground"
      >{{ node.region }}</span>
    </div>

    <div class="space-y-2.5 px-3 py-3">
      <!-- CPU / 内存 -->
      <div class="grid grid-cols-2 gap-3">
        <div>
          <div class="flex items-baseline justify-between text-[11px]">
            <span class="text-muted-foreground">CPU</span>
            <span class="font-mono font-semibold tabular-nums text-foreground">{{ cpuPct.toFixed(1) }}%</span>
          </div>
          <div class="mt-1 h-1 overflow-hidden rounded-full bg-muted">
            <div class="h-full rounded-full" :class="{ default: 'bg-muted-foreground/40', warning: 'bg-warning', error: 'bg-destructive' }[statusOf(cpuPct)]" :style="{ width: `${Math.min(cpuPct, 100)}%` }" />
          </div>
          <div class="mt-1 font-mono text-[10px] text-muted-foreground">{{ (st?.load ?? 0).toFixed(2) }} {{ (st?.load5 ?? 0).toFixed(2) }} {{ (st?.load15 ?? 0).toFixed(2) }}</div>
        </div>
        <div>
          <div class="flex items-baseline justify-between text-[11px]">
            <span class="text-muted-foreground">内存</span>
            <span class="font-mono font-semibold tabular-nums text-foreground">{{ memPct.toFixed(1) }}%</span>
          </div>
          <div class="mt-1 h-1 overflow-hidden rounded-full bg-muted">
            <div class="h-full rounded-full" :class="{ default: 'bg-muted-foreground/40', warning: 'bg-warning', error: 'bg-destructive' }[statusOf(memPct)]" :style="{ width: `${Math.min(memPct, 100)}%` }" />
          </div>
          <div class="mt-1 font-mono text-[10px] text-muted-foreground">{{ formatBytes(st?.ram ?? 0) }} / {{ formatBytes(st?.ram_total ?? 0) }}</div>
        </div>
      </div>

      <!-- 硬盘 / 网络 -->
      <div class="grid grid-cols-2 gap-3">
        <div>
          <div class="flex items-baseline justify-between text-[11px]">
            <span class="text-muted-foreground">硬盘</span>
            <span class="font-mono font-semibold tabular-nums text-foreground">{{ diskPct.toFixed(1) }}%</span>
          </div>
          <div class="mt-1 h-1 overflow-hidden rounded-full bg-muted">
            <div class="h-full rounded-full" :class="{ default: 'bg-muted-foreground/40', warning: 'bg-warning', error: 'bg-destructive' }[statusOf(diskPct)]" :style="{ width: `${Math.min(diskPct, 100)}%` }" />
          </div>
          <div class="mt-1 font-mono text-[10px] text-muted-foreground">{{ formatBytes(st?.disk ?? 0) }} / {{ formatBytes(st?.disk_total ?? 0) }}</div>
        </div>
        <div>
          <div class="text-[11px] text-muted-foreground">网络</div>
          <div class="mt-1 flex items-center gap-2 font-mono text-[11px] text-foreground">
            <span class="inline-flex items-center gap-0.5"><Icon icon="tabler:arrow-down" :width="11" class="text-muted-foreground" />{{ formatSpeed(st?.net_in ?? 0) }}</span>
            <span class="inline-flex items-center gap-0.5"><Icon icon="tabler:arrow-up" :width="11" class="text-muted-foreground" />{{ formatSpeed(st?.net_out ?? 0) }}</span>
          </div>
          <div class="mt-1 font-mono text-[10px] text-muted-foreground">运行 {{ formatUptime(st?.uptime ?? 0) }}</div>
        </div>
      </div>

      <!-- 探针延迟 -->
      <div v-if="pingList.length" class="flex flex-wrap gap-1.5 border-t border-border pt-2.5">
        <span
          v-for="p in pingList" :key="p.name"
          class="inline-flex items-center gap-1.5 rounded-full bg-muted px-2 py-0.5 text-[10px]"
        >
          <span class="size-1.5 rounded-full" :class="latencyClass(p.latest)" />
          <span class="text-muted-foreground">{{ p.name }}</span>
          <span class="font-mono tabular-nums text-foreground">{{ p.latest < 0 ? '超时' : `${p.latest.toFixed(0)}ms` }}</span>
        </span>
      </div>
    </div>
  </div>
</template>
