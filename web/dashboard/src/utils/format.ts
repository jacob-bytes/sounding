/** 格式化工具（与 ink 的 helper 对齐） */

export function formatBytes(bytes: number, decimals = 2): string {
  if (!Number.isFinite(bytes) || bytes <= 0)
    return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** i).toFixed(i === 0 ? 0 : decimals)} ${units[i]}`
}

export function formatBytesSplit(bytes: number): { value: string, unit: string } {
  const s = formatBytes(bytes)
  const [value, unit] = s.split(' ')
  return { value, unit }
}

export function formatSpeed(bytesPerSec: number): string {
  return `${formatBytes(bytesPerSec)}/s`
}

export function formatUptime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0)
    return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0)
    return `${d} 天 ${h} 小时`
  if (h > 0)
    return `${h} 小时 ${m} 分`
  return `${m} 分钟`
}

export function percent(used: number, total: number): number {
  if (!total)
    return 0
  return Math.round((used / total) * 1000) / 10
}

/** 进度条状态（0-60 中性 / 60-85 警告 / 85+ 危险——与 ink 一致） */
export function statusOf(value: number): 'default' | 'warning' | 'error' {
  if (value < 60)
    return 'default'
  if (value < 85)
    return 'warning'
  return 'error'
}

/** 延迟档位（<80 绿 / 80-150 蓝 / 150-220 琥珀 / >220 红） */
export function latencyClass(ms: number): string {
  if (ms < 0)
    return 'bg-destructive'
  if (ms < 80)
    return 'bg-success'
  if (ms < 150)
    return 'bg-primary'
  if (ms < 220)
    return 'bg-warning'
  return 'bg-destructive'
}
