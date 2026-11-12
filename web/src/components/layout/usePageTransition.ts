import { onScopeDispose, shallowRef } from 'vue'
import type { NavigationMenuItem } from '@nuxt/ui'
import type { Router } from 'vue-router'

import { mediaQueryList } from '@/utils/browserCompatibility'

function navigationPaths(items: NavigationMenuItem[]): string[] {
  return items.flatMap((item) => [
    ...(typeof item.to === 'string' ? [item.to] : []),
    ...navigationPaths(item.children ?? []),
  ])
}

function navigationPath(path: string): string {
  return path.startsWith('/admin/users/') ? '/admin/users' : path
}

/**
 * Select the local Motion direction before Vue commits the next route.
 *
 * Do not serialize router navigation on a Transition lifecycle callback.
 * Embedded WebViews can skip/cancel transition callbacks when the document is
 * backgrounded, resized, or motion preferences change, which would otherwise
 * leave the router waiting forever.
 */
export function usePageTransition(router: Router, items: () => NavigationMenuItem[]) {
  const direction = shallowRef<'forward' | 'backward'>('forward')
  const reducedMotion = mediaQueryList('(prefers-reduced-motion: reduce)')
  const wideViewport = mediaQueryList('(min-width: 900px)')
  const wide = shallowRef(wideViewport?.matches ?? false)

  const removeGuard = router.beforeResolve((to, from) => {
    if (
      to.path === from.path
      || to.meta.immersive
      || from.meta.immersive
      || reducedMotion?.matches
    ) {
      return
    }

    const paths = navigationPaths(items())
    const fromRank = paths.indexOf(navigationPath(from.path))
    const toRank = paths.indexOf(navigationPath(to.path))

    // Unknown/detail routes have no stable tab rank. Keep the default forward
    // direction instead of deriving a direction from index -1.
    direction.value = fromRank >= 0 && toRank >= 0 && toRank < fromRank
      ? 'backward'
      : 'forward'
  })

  onScopeDispose(removeGuard)
  const stopWideWatch = wideViewport
    ? (() => {
      const listener = () => { wide.value = wideViewport.matches }
      if (typeof wideViewport.addEventListener === 'function') wideViewport.addEventListener('change', listener)
      else wideViewport.addListener?.(listener)
      return () => {
        if (typeof wideViewport.removeEventListener === 'function') wideViewport.removeEventListener('change', listener)
        else wideViewport.removeListener?.(listener)
      }
    })()
    : () => undefined
  onScopeDispose(stopWideWatch)

  return { direction, wide }
}
