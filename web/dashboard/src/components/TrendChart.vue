<script setup lang="ts">
import type { NodeStatus } from '@/services/rpc'
import { computed } from 'vue'
import VChart from 'vue-echarts'
import { getLoadChartPalette } from '@/utils/chartPalette'

// ECharts 按需注册（与 ink 同款树摇）
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'

use([CanvasRenderer, LineChart, GridComponent, TooltipComponent, LegendComponent])

const props = defineProps<{
  records: NodeStatus[]
  title?: string
  height?: string
}>()

const palette = getLoadChartPalette(false)

/** 系列定义（CPU / 内存 / 网络——可扩展） */
const seriesDefs = [
  { key: 'cpu', name: 'CPU', unit: '%', color: palette.primary, area: palette.primaryAreaStrong },
  { key: 'memPct', name: '内存', unit: '%', color: palette.tertiary, area: palette.tertiaryAreaStrong },
  { key: 'diskPct', name: '硬盘', unit: '%', color: palette.quaternary, area: undefined },
  { key: 'netIn', name: '下行', unit: ' B/s', color: palette.quinary, area: undefined },
] as const

const option = computed(() => {
  const rows = props.records.map((r) => {
    const memPct = r.ram_total ? (r.ram / r.ram_total) * 100 : 0
    const diskPct = r.disk_total ? (r.disk / r.disk_total) * 100 : 0
    return { time: r.time, cpu: r.cpu, memPct, diskPct, netIn: r.net_in }
  })
  return {
    animation: false,
    grid: { left: 44, right: 16, top: 28, bottom: 28 },
    tooltip: { trigger: 'axis', confine: true },
    legend: {
      show: true,
      top: 0,
      right: 0,
      itemWidth: 10,
      itemHeight: 8,
      textStyle: { fontSize: 11, color: 'var(--color-muted-foreground)' },
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: rows.map(r => r.time.slice(11, 16)),
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { fontSize: 11, color: 'var(--color-muted-foreground)', fontFamily: 'ui-monospace, monospace', hideOverlap: true },
    },
    yAxis: {
      type: 'value',
      min: 0,
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: 'var(--color-border)', type: 'dashed' } },
      axisLabel: { fontSize: 11, color: 'var(--color-muted-foreground)', fontFamily: 'ui-monospace, monospace' },
    },
    series: seriesDefs.map(s => ({
      name: s.name,
      type: 'line',
      showSymbol: false,
      smooth: false,
      data: rows.map(r => Math.round((r as never as Record<string, number>)[s.key] * 100) / 100),
      lineStyle: { width: 1.5, color: s.color, cap: 'round' },
      areaStyle: s.area ? { color: s.area } : undefined,
    })),
  }
})
</script>

<template>
  <div class="rounded-lg border border-border bg-card">
    <div v-if="title" class="border-b border-border px-3 py-2 text-xs font-medium text-muted-foreground">{{ title }}</div>
    <div :style="{ height: height || '200px' }">
      <VChart v-if="records.length > 1" :option="option" autoresize />
      <div v-else class="flex h-full items-center justify-center text-xs text-muted-foreground">暂无历史数据</div>
    </div>
  </div>
</template>
