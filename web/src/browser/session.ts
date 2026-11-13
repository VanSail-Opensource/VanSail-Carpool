import { computed, shallowRef } from 'vue'
import { defineStore } from 'pinia'
import { browserAccessApi, type BrowserConfig, type BrowserSession } from '@/api/browserAccess'
import { ApiError } from '@/api/http'
import { localizedError } from '@/i18n'
import { useSessionStore } from '@/stores/session'

export const useBrowserSession = defineStore('browser-session', () => {
  const identity = useSessionStore()
  const state = shallowRef<BrowserSession | null>(null)
  const config = shallowRef<BrowserConfig | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const expired = shallowRef(false)
  let pending: Promise<void> | null = null
  let identityRevision = 0
  const isAdmin = computed(() => state.value?.kind === 'admin')

  function apply(next: BrowserSession): void {
    state.value = next
    if (next.kind === 'admin') {
      const sameAdministrator = identity.user?.id === next.user.id
      identity.updateSession(next)
      expired.value = false
      if (sameAdministrator) globalThis.dispatchEvent(new Event('txc:browser-session-restored'))
    } else if (identity.isAdmin) expired.value = true
  }

  function supersedePendingIdentity(): void {
    identityRevision += 1
    pending = null
    loading.value = false
  }

  async function bootstrap(force = false): Promise<void> {
    if (pending) return pending
    if (!force && state.value) return
    loading.value = true
    error.value = null
    const revision = identityRevision
    pending = (async () => {
      try {
        if (force || !config.value) config.value = await browserAccessApi.config()
        const next = await browserAccessApi.session()
        if (revision === identityRevision) apply(next)
      } catch (caught) {
        if (revision !== identityRevision) return
        if (caught instanceof ApiError && (caught.status === 401 || caught.code === 'ADMIN_REQUIRED')) {
          apply({ authenticated: false, kind: 'anonymous', miniAppUrl: config.value?.miniAppUrl ?? '' })
        } else error.value = localizedError(caught, 'browser.loadFailed')
      } finally {
        if (revision === identityRevision) {
          loading.value = false
          pending = null
        }
      }
    })()
    return pending
  }

  async function renew(): Promise<void> {
    if (!isAdmin.value || pending) return
    const revision = identityRevision
    try {
      const next = await browserAccessApi.renew()
      if (revision === identityRevision) apply(next)
    }
    catch (caught) {
      if (revision === identityRevision && caught instanceof ApiError && (caught.status === 401 || caught.status === 403)) await bootstrap(true)
    }
  }

  async function logout(): Promise<void> {
    await browserAccessApi.logout()
    supersedePendingIdentity()
    identity.clear()
    expired.value = false
    state.value = null
    await bootstrap(true)
  }

  async function exchange(token: string, signal?: AbortSignal): Promise<string> {
    supersedePendingIdentity()
    const revision = identityRevision
    const assertCurrent = (): void => {
      if (signal?.aborted || revision !== identityRevision) throw new DOMException('BROWSER_EXCHANGE_SUPERSEDED', 'AbortError')
    }
    assertCurrent()
    if (!config.value) {
      const nextConfig = await browserAccessApi.config(signal)
      assertCurrent()
      config.value = nextConfig
    }
    const next = await browserAccessApi.exchange(token, signal)
    assertCurrent()
    supersedePendingIdentity()
    apply(next)
    return next.kind === 'admin' ? next.returnTo ?? '/admin/settings' : '/'
  }

  return { state, config, loading, error, expired, isAdmin, bootstrap, renew, logout, exchange }
})
