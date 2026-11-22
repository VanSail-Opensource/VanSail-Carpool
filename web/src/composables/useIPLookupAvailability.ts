import { inject, onScopeDispose, provide, shallowRef, watch, type InjectionKey } from 'vue'
import { ipLookupApi } from '@/api/ipLookup'
import { useSessionStore } from '@/stores/session'

interface Availability { enabled: ReturnType<typeof shallowRef<boolean>>; refresh: () => Promise<void> }
const key: InjectionKey<Availability> = Symbol('ip-lookup-availability')

export function useIPLookupAvailability(eligible: () => boolean = () => true): Availability {
  const shared = inject(key, null)
  if (shared) return shared
  const session = useSessionStore()
  const enabled = shallowRef(false)
  let version = 0
  let disposed = false
  async function refresh(): Promise<void> {
    const requestVersion = ++version
    const owner = session.user?.id
    if (!owner || !eligible()) { enabled.value = false; return }
    try {
      const value = await ipLookupApi.state()
      if (!disposed && requestVersion === version && session.user?.id === owner) enabled.value = value.enabled
    } catch {
      if (!disposed && requestVersion === version) enabled.value = false
    }
  }
  watch(() => eligible() ? session.user?.id : undefined, () => { enabled.value = false; void refresh() }, { immediate: true })
  onScopeDispose(() => { disposed = true; version++ })
  const context = { enabled, refresh }
  provide(key, context)
  return context
}
