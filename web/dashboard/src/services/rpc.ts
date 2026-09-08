/** sounding JSON-RPC 2.0 客户端（HTTP + WebSocket，契约与 sounding-server 对齐） */

export interface NodeStatusPing {
  name: string
  latest: number
  avg: number
  tail: number
  loss: number
  min: number
  max: number
}

export interface NodeStatus {
  client: string
  time: string
  cpu: number
  ram: number
  ram_total: number
  swap: number
  swap_total: number
  load: number
  load5: number
  load15: number
  temp: number
  disk: number
  disk_total: number
  net_in: number
  net_out: number
  net_total_up: number
  net_total_down: number
  process: number
  online: boolean
  uptime: number
  ping?: Record<string, NodeStatusPing>
}

export interface Client {
  uuid: string
  name: string
  cpu_name: string
  arch: string
  os: string
  region: string
  cpu_cores: number
  mem_total: number
  disk_total: number
  price: number
  currency: string
  expired_at: string
  tags: string
  public_remark: string
}

const base = (import.meta.env.VITE_API_BASE as string | undefined) || ''

let requestId = 0

async function call<T>(method: string, params: Record<string, unknown> = {}): Promise<T> {
  const res = await fetch(`${base}/rpc2`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: ++requestId, method, params }),
  })
  const data = await res.json()
  if (data.error)
    throw new Error(data.error.message)
  return data.result as T
}

export const rpc = {
  getNodes: () => call<Record<string, Client>>('getNodes'),
  getNodesLatestStatus: () => call<Record<string, NodeStatus>>('getNodesLatestStatus'),
  getNodeRecentStatus: (uuid: string, limit = 150) => call<{ count: number, records: NodeStatus[] }>('getNodeRecentStatus', { client: uuid, limit }),
  ping: () => call<string>('ping'),
}

/** WebSocket 实时推送（sounding.push——statuses + nodes） */
export interface PushPayload {
  statuses: Record<string, NodeStatus>
  nodes: Record<string, Client>
  time: string
}

export function connectPush(onData: (p: PushPayload) => void, onState?: (connected: boolean) => void): () => void {
  let ws: WebSocket | null = null
  let closed = false
  let retry = 0

  function connect() {
    if (closed)
      return
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    ws = new WebSocket(`${proto}//${location.host}/rpc2`)
    ws.onopen = () => { retry = 0; onState?.(true) }
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data)
        if (msg.method === 'sounding.push' && msg.params)
          onData(msg.params as PushPayload)
      }
      catch { /* 忽略非法消息 */ }
    }
    ws.onclose = () => {
      onState?.(false)
      ws = null
      if (!closed && retry++ < 10)
        setTimeout(connect, Math.min(3000 * retry, 15000))
    }
    ws.onerror = () => ws?.close()
  }
  connect()
  return () => { closed = true; ws?.close() }
}
