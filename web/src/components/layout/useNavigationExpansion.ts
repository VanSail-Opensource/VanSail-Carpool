import { shallowRef, watch } from 'vue'
import type { NavigationMenuItem } from '@nuxt/ui'

/** Apply defaults when an eligible group arrives, without undoing user toggles. */
export function useNavigationExpansion(items: () => NavigationMenuItem[]) {
  const expanded = shallowRef<string[]>([])
  let knownGroups = new Set<string>()

  watch(items, next => {
    const groups = next.filter((item): item is NavigationMenuItem & { value: string } =>
      Boolean(item.children?.length) && typeof item.value === 'string')
    const available = new Set(groups.map(item => item.value))
    const nextExpanded = new Set(expanded.value.filter(value => available.has(value)))
    for (const item of groups) {
      if (!knownGroups.has(item.value) && item.defaultOpen) nextExpanded.add(item.value)
    }
    knownGroups = available
    const values = [...nextExpanded]
    if (values.length !== expanded.value.length || values.some((value, index) => value !== expanded.value[index])) expanded.value = values
  }, { immediate: true, flush: 'sync' })

  return expanded
}
