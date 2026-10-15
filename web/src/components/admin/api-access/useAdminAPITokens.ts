import { onMounted, onScopeDispose, shallowRef } from 'vue'
import { adminAPITokensApi, type PanelAPIToken, type PanelAPITokenIssued, type PanelAPITokenWrite } from '@/api/adminAPITokens'
import { localizedError } from '@/i18n'

export function useAdminAPITokens() {
  const items = shallowRef<PanelAPIToken[]>([])
  const issued = shallowRef<PanelAPITokenIssued | null>(null)
  const loading = shallowRef(true)
  const busy = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const loadError = shallowRef<string | null>(null)
  let active = true
  let generation = 0
  let secretGeneration = 0
  const dismiss = () => { issued.value = null; secretGeneration++ }
  async function load(): Promise<void> {
    const current = ++generation
    loading.value = true
    try {
      const result = await adminAPITokensApi.list()
      if (active && current === generation) { items.value = result.items; loadError.value = null }
    } catch (caught) { if (active && current === generation) loadError.value = localizedError(caught, 'apiTokens.loadFailed') }
    finally { if (active && current === generation) loading.value = false }
  }
  async function create(value: PanelAPITokenWrite): Promise<void> {
    if (busy.value) return
    generation++; loading.value = false
    busy.value = true; error.value = null; dismiss()
    const secretVersion = secretGeneration
    try {
      const result = await adminAPITokensApi.create(value)
      if (active) {
        if (secretVersion === secretGeneration) issued.value = result
        items.value = [result.item, ...items.value]
      }
    } catch (caught) { if (active) error.value = localizedError(caught, 'apiTokens.createFailed') }
    finally { if (active) busy.value = false }
  }
  async function revoke(id: string): Promise<void> {
    if (busy.value) return
    busy.value = true; error.value = null
    try {
      await adminAPITokensApi.revoke(id)
      if (active) {
        if (issued.value?.item.id === id) dismiss()
        items.value = items.value.map(item => item.id === id ? { ...item, revokedAt: new Date().toISOString() } : item)
        await load()
      }
    } catch (caught) { if (active) error.value = localizedError(caught, 'apiTokens.revokeFailed') }
    finally { if (active) busy.value = false }
  }
  onMounted(() => { window.addEventListener('pagehide', dismiss); void load() })
  onScopeDispose(() => { active = false; generation++; dismiss(); window.removeEventListener('pagehide', dismiss) })
  return { items, issued, loading, busy, error, loadError, load, create, revoke, dismiss }
}
