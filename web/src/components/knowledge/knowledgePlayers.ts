import type { Component } from 'vue'
import { isKnowledgeLesson, type KnowledgeTopicId } from './knowledgeTopics'

type PlayerKind = 'lesson' | 'billing' | 'routing'
type PlayerModule = { default: Component }
const loaders: Record<PlayerKind, () => Promise<PlayerModule>> = {
  lesson: () => import('./KnowledgePlayer.vue'),
  billing: () => import('@/components/catalog/BillingDemo.vue'),
  routing: () => import('@/components/diy/RoutingDemo.vue'),
}
const pending = new Map<PlayerKind, Promise<PlayerModule>>()

/** Intent and play share one import; a failed attempt is never kept as a cache hit. */
export function loadKnowledgePlayer(topic: KnowledgeTopicId): Promise<PlayerModule> {
  const kind: PlayerKind = isKnowledgeLesson(topic) ? 'lesson' : topic === 'routing' ? 'routing' : 'billing'
  const existing = pending.get(kind)
  if (existing) return existing
  const work = loaders[kind]().catch(error => { pending.delete(kind); throw error })
  pending.set(kind, work)
  return work
}

export async function preloadKnowledgePlayer(topic: KnowledgeTopicId): Promise<void> {
  const connection = (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection
  if (document.hidden || connection?.saveData || ['slow-2g', '2g'].includes(connection?.effectiveType ?? '')) return
  await loadKnowledgePlayer(topic).catch(() => undefined)
}
