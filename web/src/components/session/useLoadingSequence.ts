import { computed, shallowRef, watch } from 'vue'
import type { SessionStatus } from '@/stores/session'

/** Readiness owns access. Decorative playback never holds a ready session. */
export function useLoadingSequence(status: () => SessionStatus, authenticated: () => boolean = () => false, routeReady: () => boolean = () => true) {
  const settled = shallowRef(false)
  const loading = computed(() => status() === 'idle' || status() === 'loading')
  watch(status, next => {
    if (next === 'ready' || next === 'error') settled.value = true
  }, { immediate: true })
  const showing = computed(() => (loading.value && !authenticated()) || (status() === 'ready' && !routeReady()))
  const introducing = computed(() => !settled.value)
  return { loading, showing, introducing }
}
