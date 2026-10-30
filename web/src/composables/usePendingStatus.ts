import { shallowRef, watch } from 'vue'

/** One factual status replacement while work is pending; never delays readiness. */
export function usePendingStatus(pending: () => boolean) {
  const slow = shallowRef(false)
  watch(pending, (active, _previous, cleanup) => {
    slow.value = false
    if (!active) return
    const timer = globalThis.setTimeout(() => { slow.value = true }, 5000)
    cleanup(() => globalThis.clearTimeout(timer))
  }, { immediate: true })
  return { slow }
}
