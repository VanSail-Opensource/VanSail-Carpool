import { computed, shallowRef } from 'vue'
import { defineStore } from 'pinia'

import { api, ApiError } from '@/api/client'
import { setCacheSession } from '@/api/cache/session'
import { preloadSession, stopSessionPreload } from '@/api/cache/preload'
import type { Session } from '@/api/types'
import { localizedError, t } from '@/i18n'
import { isBrowserExperience } from '@/utils/experience'

export type SessionStatus = 'idle' | 'loading' | 'ready' | 'error'

export const useSessionStore = defineStore('session', () => {
  const session = shallowRef<Session | null>(null)
  const status = shallowRef<SessionStatus>('idle')
  const error = shallowRef<string | null>(null)
  const errorCode = shallowRef<string | null>(null)
  let revision = 0
  let pending: Promise<void> | null = null

  const user = computed(() => session.value?.user ?? null)
  const isAuthenticated = computed(() => session.value !== null)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const onboardingComplete = computed(() => user.value?.onboardingState === 'complete')

  function bootstrap(force = false): Promise<void> {
    if (!force && pending) return pending
    if (!force && status.value === 'ready') return Promise.resolve()
    const current = ++revision
    status.value = 'loading'
    error.value = null
    errorCode.value = null
    pending = loadSession(current)
    return pending
  }

  async function loadSession(current: number): Promise<void> {
    try {
      let next: Session
      try {
        next = await api.getMe()
      } catch (caught) {
        if (current !== revision) return
        if (!(caught instanceof ApiError) || caught.status !== 401) throw caught
        if (isBrowserExperience()) throw caught
        const { getTelegramInitData, isTelegramWebAppDetected } = await import('@/utils/telegram')
        const initData = await getTelegramInitData()
        if (current !== revision) return
        if (!initData) {
          throw new Error(isTelegramWebAppDetected() ? t('auth.telegramLoading') : t('auth.openInTelegram'), { cause: caught })
        }
        next = await api.authTelegram(initData)
      }
      if (current === revision) updateSession(next)
    } catch (caught) {
      if (current !== revision) return
      stopSessionPreload()
      setCacheSession(null)
      session.value = null
      status.value = 'error'
      error.value = localizedError(caught, 'auth.authenticationFailed')
      errorCode.value = caught instanceof ApiError ? caught.code : null
    } finally {
      if (current === revision) pending = null
    }
  }

  function updateSession(next: Session): void {
    revision += 1
    pending = null
    const changed = setCacheSession(`${isBrowserExperience() ? 'browser' : 'miniapp'}:${next.user.id}:${next.user.role}:${next.user.onboardingState}:${next.captcha?.required === true}`)
    session.value = next
    status.value = 'ready'
    error.value = null
    errorCode.value = null
    if (next.captcha?.required) stopSessionPreload()
    else if (changed && !isBrowserExperience()) preloadSession(next)
  }

  function clear(): void {
    revision += 1
    pending = null
    globalThis.dispatchEvent(new Event('txc:session-cleared'))
    stopSessionPreload()
    setCacheSession(null)
    session.value = null
    status.value = 'idle'
    error.value = null
    errorCode.value = null
  }

  return {
    session,
    status,
    error,
    errorCode,
    user,
    isAuthenticated,
    isAdmin,
    onboardingComplete,
    bootstrap,
    updateSession,
    clear,
  }
})
