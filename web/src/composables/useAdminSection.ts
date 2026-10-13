import { onMounted, onScopeDispose, readonly, shallowRef } from 'vue'

import { api } from '@/api/client'
import { restoreCached } from '@/api/cache/restore'
import type { QueryValue } from '@/api/http'
import type { AdminResource, Paginated } from '@/api/types'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

function extractItems<T>(payload: Paginated<T> | T[] | { items: T[] }): T[] {
  if (Array.isArray(payload)) return payload
  return payload.items
}

export function useAdminSection<T>(resource: AdminResource, options: { immediate?: boolean } = {}) {
  const items = shallowRef<T[]>([])
  const loading = shallowRef(options.immediate !== false)
  const busy = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const nextCursor = shallowRef<string | null>(null)
  const latestLoad = createLatestRequest()
	type Query = Record<string, QueryValue>
  let lastQuery: Query = {}

  async function load(query?: Query, options: { append?: boolean } = {}): Promise<void> {
    const append = options.append === true
    if (append && !nextCursor.value) return
    const requestQuery = append
      ? { ...lastQuery, cursor: nextCursor.value ?? undefined }
      : { ...(query ?? lastQuery), cursor: undefined }
    if (!append) lastQuery = requestQuery
    const token = latestLoad.begin()
    loading.value = append || !restoreCached<Paginated<T> | T[] | { items: T[] }>(`/api/v1/admin/${resource}`, payload => {
      items.value = extractItems(payload)
      nextCursor.value = Array.isArray(payload) || !('page' in payload) ? null : payload.page?.nextCursor ?? null
    }, { query: requestQuery })
    error.value = null
    try {
      const payload = await api.getAdminResource<Paginated<T> | T[] | { items: T[] }>(resource, requestQuery)
      if (!latestLoad.isCurrent(token)) return
      const nextItems = extractItems(payload)
      items.value = append ? [...items.value, ...nextItems] : nextItems
      nextCursor.value = Array.isArray(payload) || !("page" in payload) ? null : payload.page?.nextCursor ?? null
    } catch (caught) {
      if (!latestLoad.isCurrent(token)) return
      error.value = localizedError(caught, 'errors.adminLoad')
    } finally {
      if (latestLoad.isCurrent(token)) loading.value = false
    }
  }

  function loadMore(): Promise<void> {
    return load(undefined, { append: true })
  }

  async function perform(task: () => Promise<unknown>): Promise<boolean> {
    busy.value = true
    error.value = null
    try {
      await task()
      await load(lastQuery)
      return true
    } catch (caught) {
      error.value = localizedError(caught, 'errors.adminAction')
      return false
    } finally {
      busy.value = false
    }
  }

  function create(body: unknown): Promise<boolean> {
    return perform(() => api.createAdminResource(resource, body))
  }

  function update(id: string, body: unknown): Promise<boolean> {
    return perform(() => api.updateAdminResource(resource, id, body))
  }

  function remove(id: string): Promise<boolean> {
    return perform(() => api.deleteAdminResource(resource, id))
  }

  if (options.immediate !== false) onMounted(() => void load())
  onScopeDispose(latestLoad.dispose)

  return {
    items: readonly(items),
    loading: readonly(loading),
    busy: readonly(busy),
    error: readonly(error),
    nextCursor: readonly(nextCursor),
    load,
    loadMore,
    perform,
    create,
    update,
    remove,
  }
}
