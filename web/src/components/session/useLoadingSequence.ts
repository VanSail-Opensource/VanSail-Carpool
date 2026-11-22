import { computed } from 'vue'
import type { SessionStatus } from '@/stores/session'

/** Derive cover visibility from real session and route readiness, never a timer. */
export function useLoadingSequence(status: () => SessionStatus, authenticated: () => boolean, routeReady: () => boolean) {
  return computed(() => {
    const state = status()
    return ((state === 'idle' || state === 'loading') && !authenticated()) || (state === 'ready' && !routeReady())
  })
}
