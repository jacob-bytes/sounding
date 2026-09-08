import type { Client, NodeStatus } from '@/services/rpc'
import { onUnmounted, ref } from 'vue'
import { connectPush, rpc } from '@/services/rpc'

/** 实时数据（WS 推送优先，失败回退轮询） */
export function useRealtime() {
  const nodes = ref<Record<string, Client>>({})
  const statuses = ref<Record<string, NodeStatus>>({})
  const connected = ref(false)
  const updatedAt = ref<string>('')

  const disconnect = connectPush(
    (p) => {
      nodes.value = p.nodes || {}
      statuses.value = p.statuses || {}
      updatedAt.value = p.time
    },
    v => connected.value = v,
  )

  // 首屏 + 轮询兜底（WS 不可用时）
  async function refresh() {
    const [n, s] = await Promise.all([rpc.getNodes(), rpc.getNodesLatestStatus()])
    nodes.value = n
    statuses.value = s
    updatedAt.value = new Date().toISOString()
  }
  void refresh()
  const timer = setInterval(() => {
    if (!connected.value)
      void refresh()
  }, 10000)

  onUnmounted(() => {
    disconnect()
    clearInterval(timer)
  })

  return { nodes, statuses, connected, updatedAt, refresh }
}
