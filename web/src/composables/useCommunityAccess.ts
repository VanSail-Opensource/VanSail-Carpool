import { computed, inject, onScopeDispose, provide, readonly, shallowRef, watch, type ComputedRef, type InjectionKey, type ShallowRef } from 'vue'

import { api } from '@/api/client'
import { useSessionStore } from '@/stores/session'

interface CommunityAccessContext {
  activeCombo: Readonly<ShallowRef<boolean>>
  positiveBalance: Readonly<ShallowRef<boolean>>
  hasAccess: ComputedRef<boolean>
  loading: Readonly<ShallowRef<boolean>>
  refresh: () => Promise<void>
}

const communityAccessKey: InjectionKey<CommunityAccessContext> = Symbol('community-access')

function positive(minor: string | undefined): boolean {
  try {
    return BigInt(minor ?? '0') > 0n
  } catch {
    return false
  }
}

function createCommunityAccess(): CommunityAccessContext {
  const session = useSessionStore()
  const activeCombo = shallowRef(false)
  const positiveBalance = shallowRef(false)
  const loading = shallowRef(true)
  const hasAccess = computed(() => activeCombo.value || positiveBalance.value)
  let disposed = false
  let revision = 0
  let pending: Promise<void> | null = null

  function refresh(): Promise<void> {
    const owner = session.user?.id
    if (disposed || !owner) return Promise.resolve()
    if (pending) return pending
    const current = revision
    const valid = () => !disposed && current === revision && session.user?.id === owner
    const request = Promise.all([
      api.getCommunityAccess()
        .then((response) => {
          if (valid()) activeCombo.value = response.activeCombo
        })
        .catch(() => {
          if (valid()) activeCombo.value = false
        }),
      api.getBalance()
        .then((response) => {
          if (valid()) positiveBalance.value = positive(response.balance?.minor)
        })
        .catch(() => {
          if (valid()) positiveBalance.value = false
        }),
    ])
      .then(() => undefined)
      .finally(() => {
        if (valid()) loading.value = false
        if (pending === request) pending = null
      })
    pending = request
    return request
  }

  function clear(): void {
    revision++
    pending = null
    activeCombo.value = false
    positiveBalance.value = false
    loading.value = false
  }
  watch(() => session.user?.id ?? null, owner => {
    clear()
    if (owner) { loading.value = true; void refresh() }
  }, { immediate: true, flush: 'sync' })
  onScopeDispose(() => { disposed = true; clear() })

  return { activeCombo: readonly(activeCombo), positiveBalance: readonly(positiveBalance), hasAccess, loading: readonly(loading), refresh }
}

export function useCommunityAccess(): CommunityAccessContext {
  const provided = inject(communityAccessKey, null)
  if (provided) return provided
  const context = createCommunityAccess()
  provide(communityAccessKey, context)
  return context
}
