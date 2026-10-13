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

  const user = computed(() => session.value?.user ?? null)
  const isAuthenticated = computed(() => session.value !== null)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const onboardingComplete = computed(() => user.value?.onboardingState === 'complete')

  async function bootstrap(force = false): Promise<void> {
    if (!force && (status.value === 'loading' || status.value === 'ready')) return
    status.value = 'loading'
    error.value = null

    try {
      try {
        session.value = await api.getMe()
      } catch (caught) {
        if (!(caught instanceof ApiError) || caught.status !== 401) throw caught
        if (isBrowserExperience()) throw caught
        const { getTelegramInitData, isTelegramWebAppDetected } = await import('@/utils/telegram')
        const initData = await getTelegramInitData()
        if (!initData) {
          throw new Error(isTelegramWebAppDetected() ? t('auth.telegramLoading') : t('auth.openInTelegram'), { cause: caught })
        }
        session.value = await api.authTelegram(initData)
      }
      updateSession(session.value)
    } catch (caught) {
      stopSessionPreload()
      setCacheSession(null)
      session.value = null
      status.value = 'error'
      error.value = localizedError(caught, 'auth.authenticationFailed')
    }
  }

  function updateSession(next: Session): void {
    const changed = setCacheSession(`${isBrowserExperience() ? 'browser' : 'miniapp'}:${next.user.id}:${next.user.role}:${next.user.onboardingState}:${next.captcha?.required === true}`)
    session.value = next
    status.value = 'ready'
    error.value = null
    if (next.captcha?.required) stopSessionPreload()
    else if (changed && !isBrowserExperience()) preloadSession(next)
  }

  function clear(): void {
    stopSessionPreload()
    setCacheSession(null)
    session.value = null
    status.value = 'idle'
    error.value = null
  }

  return {
    session,
    status,
    error,
    user,
    isAuthenticated,
    isAdmin,
    onboardingComplete,
    bootstrap,
    updateSession,
    clear,
  }
})
