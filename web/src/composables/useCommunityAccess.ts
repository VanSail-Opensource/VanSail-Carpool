import { computed, inject, onMounted, onScopeDispose, provide, readonly, shallowRef, type ComputedRef, type InjectionKey, type ShallowRef } from 'vue'

import { api } from '@/api/client'

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
  const activeCombo = shallowRef(false)
  const positiveBalance = shallowRef(false)
  const loading = shallowRef(true)
  const hasAccess = computed(() => activeCombo.value || positiveBalance.value)
  let disposed = false
  let pending: Promise<void> | null = null

  function refresh(): Promise<void> {
    if (pending) return pending
    const request = Promise.all([
      api.getCommunityAccess()
        .then((response) => {
          if (!disposed) activeCombo.value = response.activeCombo
        })
        .catch(() => {
          if (!disposed) activeCombo.value = false
        }),
      api.getBalance()
        .then((response) => {
          if (!disposed) positiveBalance.value = positive(response.balance?.minor)
        })
        .catch(() => {
          if (!disposed) positiveBalance.value = false
        }),
    ])
      .then(() => undefined)
      .finally(() => {
        if (!disposed) loading.value = false
        if (pending === request) pending = null
      })
    pending = request
    return request
  }

  onMounted(() => void refresh())
  onScopeDispose(() => { disposed = true })

  return { activeCombo: readonly(activeCombo), positiveBalance: readonly(positiveBalance), hasAccess, loading: readonly(loading), refresh }
}

export function useCommunityAccess(): CommunityAccessContext {
  const provided = inject(communityAccessKey, null)
  if (provided) return provided
  const context = createCommunityAccess()
  provide(communityAccessKey, context)
  return context
}
