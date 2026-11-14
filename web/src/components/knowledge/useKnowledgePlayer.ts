import { onScopeDispose, shallowRef } from 'vue'
import { loadKnowledgePlayer } from './knowledgePlayers'
import type { KnowledgeTopicId } from './knowledgeTopics'

/** Import readiness belongs to the page; the mounted player owns its playback. */
export function useKnowledgePlayer() {
  const selected = shallowRef<KnowledgeTopicId>('networks')
  const open = shallowRef(false)
  const trigger = shallowRef<globalThis.HTMLElement | null>(null)
  const loadingTopic = shallowRef<KnowledgeTopicId | null>(null)
  const errorTopic = shallowRef<KnowledgeTopicId | null>(null)
  let generation = 0
  let timeout: ReturnType<typeof globalThis.setTimeout> | undefined

  function clearTimeout(): void {
    if (timeout !== undefined) globalThis.clearTimeout(timeout)
    timeout = undefined
  }
  async function play(topic: KnowledgeTopicId, event: globalThis.MouseEvent): Promise<void> {
    const request = ++generation
    clearTimeout()
    selected.value = topic
    trigger.value = event.currentTarget instanceof globalThis.HTMLElement ? event.currentTarget : null
    open.value = false
    loadingTopic.value = topic
    errorTopic.value = null
    timeout = globalThis.setTimeout(() => {
      if (request !== generation) return
      generation++
      timeout = undefined
      loadingTopic.value = null
      errorTopic.value = topic
    }, 30000)
    try {
      await loadKnowledgePlayer(topic)
      if (request !== generation) return
      clearTimeout()
      loadingTopic.value = null
      open.value = true
    } catch {
      if (request !== generation) return
      clearTimeout()
      loadingTopic.value = null
      errorTopic.value = topic
    }
  }
  onScopeDispose(() => { generation++; clearTimeout() })
  return { selected, open, trigger, loadingTopic, errorTopic, play }
}
