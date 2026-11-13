import { onMounted, onScopeDispose, readonly, shallowRef, watch } from 'vue'

import { api } from '@/api/client'
import { restoreRef } from '@/api/cache/restore'
import type { CommunityMembership, CommunitySpace } from '@/api/types'
import { localizedError } from '@/i18n'
import { useSessionStore } from '@/stores/session'
import { getTelegramWebApp, notifyHaptic, openExternalLink } from '@/utils/telegram'

export function useCommunityMembership() {
  const sessionStore = useSessionStore()
  const membership = shallowRef<CommunityMembership | null>(null)
  const loading = shallowRef(true)
  const refreshing = shallowRef(false)
  const joining = shallowRef<readonly CommunitySpace[]>([])
  const error = shallowRef<string | null>(null)
  let disposed = false
  let mounted = false
  let ownerRevision = 0
  let requestRevision = 0
  let telegram: TelegramWebApp | undefined

  async function load(initial = false): Promise<void> {
    const owner = sessionStore.user?.id
    if (disposed) return
    if (!owner) { loading.value = false; return }
    const revision = ++requestRevision
    const current = () => !disposed && revision === requestRevision && sessionStore.user?.id === owner
    if (initial) loading.value = !restoreRef('/api/v1/community/membership/check', membership, { method: 'POST' })
    else refreshing.value = true
    error.value = null
    try {
      const next = await api.checkCommunityMembership()
      if (!current() || next.user.id !== owner) return
      membership.value = next
      if (sessionStore.session) sessionStore.updateSession({ ...sessionStore.session, user: next.user })
    } catch (caught) {
      if (current()) error.value = localizedError(caught, 'community.errorDescription')
    } finally {
      if (current()) { loading.value = false; refreshing.value = false }
    }
  }

  async function join(space: CommunitySpace): Promise<void> {
    const owner = sessionStore.user?.id
    if (disposed || !owner || joining.value.includes(space)) return
    const revision = ownerRevision
    const current = () => !disposed && revision === ownerRevision && sessionStore.user?.id === owner
    joining.value = [...joining.value, space]
    error.value = null
    try {
      const invite = await api.createCommunityInvite(space)
      if (current()) openExternalLink(invite.url)
    } catch (caught) {
      if (!current()) return
      error.value = localizedError(caught, 'community.errorDescription')
      notifyHaptic('error')
    } finally {
      if (current()) joining.value = joining.value.filter((value) => value !== space)
    }
  }

  const refreshAfterActivation = (): void => { void load() }
  watch(() => sessionStore.user?.id, (owner) => {
    ownerRevision++
    requestRevision++
    membership.value = null
    error.value = null
    loading.value = Boolean(owner)
    refreshing.value = false
    joining.value = []
    if (mounted && owner) void load(true)
  }, { flush: 'sync' })

  onMounted(() => {
    mounted = true
    telegram = getTelegramWebApp()
    telegram?.onEvent?.('activated', refreshAfterActivation)
    void load(true)
  })
  onScopeDispose(() => {
    disposed = true
    telegram?.offEvent?.('activated', refreshAfterActivation)
  })

  return {
    membership: readonly(membership),
    loading: readonly(loading),
    refreshing: readonly(refreshing),
    joining: readonly(joining),
    error: readonly(error),
    load,
    join,
  }
}
