import { shallowReadonly, shallowRef } from 'vue'
import type { Router } from 'vue-router'
import { beginRouteRecovery, completeRouteRecovery, isRouteChunkError } from '@/router/recovery'

// A failed client navigation has not committed its URL. Replace with the failed
// destination so the one permitted document retry preserves the intended page.
export function installBrowserRouteRecovery(
  router: Pick<Router, 'onError' | 'afterEach'>,
  replace: (target: string) => void = target => globalThis.location.replace(target),
) {
  const error = shallowRef<unknown>(null)
  const target = shallowRef<string | null>(null)
  router.onError((caught, to) => {
    error.value = caught
    target.value = to.fullPath
    if (!isRouteChunkError(caught) || !beginRouteRecovery(to.fullPath)) return
    try { replace(to.fullPath) }
    catch { /* Retain the failure for explicit retry if navigation is blocked. */ }
  })
  router.afterEach((to, _from, failure) => {
    if (failure) return
    error.value = null
    target.value = null
    completeRouteRecovery(to.fullPath)
  })
  return { error: shallowReadonly(error), target: shallowReadonly(target) }
}
