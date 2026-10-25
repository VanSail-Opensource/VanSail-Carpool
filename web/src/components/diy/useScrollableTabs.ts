import { onScopeDispose, shallowRef, watch, type Ref } from 'vue'

/** Keep the native tab row scrollable without scrolling the page on selection. */
export function useScrollableTabs(root: Ref<HTMLElement | null>) {
  const before = shallowRef(false), after = shallowRef(false)
  let list: HTMLElement | null = null
  let observer: ResizeObserver | undefined
  function measure() {
    before.value = (list?.scrollLeft ?? 0) > 1
    after.value = !!list && list.scrollLeft + list.clientWidth < list.scrollWidth - 1
  }
  function revealSelected() {
    const selected = list?.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')
    if (!list || !selected) return
    const bounds = list.getBoundingClientRect(), tab = selected.getBoundingClientRect()
    const delta = tab.left < bounds.left ? tab.left - bounds.left : tab.right > bounds.right ? tab.right - bounds.right : 0
    if (delta) list.scrollBy({ left: delta, behavior: globalThis.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth' })
    measure()
  }
  function detach() { observer?.disconnect(); list?.removeEventListener('scroll', measure); list = null }
  watch(root, value => {
    detach()
    list = value?.querySelector<HTMLElement>('[role="tablist"]') ?? null
    if (!list) return
    list.addEventListener('scroll', measure, { passive: true })
    observer = new ResizeObserver(() => { revealSelected(); measure() })
    observer.observe(list)
    for (const tab of list.querySelectorAll('[role="tab"]')) observer.observe(tab)
    revealSelected()
  }, { flush: 'post' })
  onScopeDispose(detach)
  return { before, after, revealSelected }
}
