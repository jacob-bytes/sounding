/** className 合并（shadcn 风格——轻量实现，避免引入 clsx/tailwind-merge 依赖） */
export function cn(...classes: Array<string | Record<string, boolean> | undefined | null | false>): string {
  const out: string[] = []
  for (const c of classes) {
    if (!c)
      continue
    if (typeof c === 'string')
      out.push(c)
    else
      for (const [k, v] of Object.entries(c)) if (v) out.push(k)
  }
  return out.join(' ')
}
