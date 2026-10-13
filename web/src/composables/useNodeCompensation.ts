import { onMounted, shallowRef } from 'vue'

import { compensationApi } from '@/api/compensation'
import { restoreCached, restoreRef } from '@/api/cache/restore'
import type { NodeCompensationConfig, NodeCompensationConfigWrite, NodeCompensationEvent, NodeCompensationStatus } from '@/api/contracts/compensation'
import { localizedError } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'

export function useNodeCompensation() {
  const config = shallowRef<NodeCompensationConfig | null>(null)
  const events = shallowRef<NodeCompensationEvent[]>([])
  const status = shallowRef<NodeCompensationStatus | ''>('')
  const nextCursor = shallowRef<string | null>(null)
  const loading = shallowRef(true)
  const busy = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const reviewKeys = new Map<string, string>()

  async function load(): Promise<void> {
    loading.value = ![
      restoreRef('/api/v1/admin/node-compensation/config', config),
      restoreCached<Awaited<ReturnType<typeof compensationApi.events>>>('/api/v1/admin/node-compensation/events', page => {
        events.value = page.items
        nextCursor.value = page.nextCursor
      }, { query: { status: status.value || undefined, limit: 25 } }),
    ].every(Boolean)
    error.value = null
    try {
      const [nextConfig, page] = await Promise.all([compensationApi.config(), compensationApi.events(status.value)])
      config.value = nextConfig
      events.value = page.items
      nextCursor.value = page.nextCursor
    } catch (caught) {
      error.value = localizedError(caught, 'adminCompensation.loadFailed')
    } finally {
      loading.value = false
    }
  }

  async function loadMore(): Promise<void> {
    if (!nextCursor.value || busy.value) return
    busy.value = true
    try {
      const page = await compensationApi.events(status.value, nextCursor.value)
      events.value = [...events.value, ...page.items]
      nextCursor.value = page.nextCursor
    } catch (caught) {
      error.value = localizedError(caught, 'adminCompensation.loadFailed')
    } finally {
      busy.value = false
    }
  }

  async function saveConfig(input: NodeCompensationConfigWrite): Promise<boolean> {
    busy.value = true
    error.value = null
    try {
      config.value = await compensationApi.saveConfig(input)
      return true
    } catch (caught) {
      error.value = localizedError(caught, 'adminCompensation.saveFailed')
      return false
    } finally {
      busy.value = false
    }
  }

  async function review(event: NodeCompensationEvent, action: 'approve' | 'dismiss', minutes: number, reason: string): Promise<boolean> {
    const identity = `${action}:${event.id}`
    const key = reviewKeys.get(identity) ?? createUuid()
    reviewKeys.set(identity, key)
    busy.value = true
    error.value = null
    try {
      const body = { revision: event.revision, extensionMinutes: minutes, reason: reason.trim() }
      const updated = action === 'approve'
        ? await compensationApi.approve(event.id, body, key)
        : await compensationApi.dismiss(event.id, body, key)
      events.value = events.value.map((item) => item.id === updated.id ? updated : item)
      reviewKeys.delete(identity)
      return true
    } catch (caught) {
      error.value = localizedError(caught, 'adminCompensation.reviewFailed')
      return false
    } finally {
      busy.value = false
    }
  }

  async function changeStatus(value: NodeCompensationStatus | ''): Promise<void> {
    status.value = value
    await load()
  }

  onMounted(load)
  return { config, events, status, nextCursor, loading, busy, error, load, loadMore, saveConfig, review, changeStatus }
}
