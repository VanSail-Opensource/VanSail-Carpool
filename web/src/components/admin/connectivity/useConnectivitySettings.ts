import { computed, onScopeDispose, reactive, shallowRef, watch, type ShallowRef } from 'vue'
import { connectivityApi, type ConnectivitySnapshot, type ConnectivityUser, type ConnectivityConfig } from '@/api/connectivity'
import { mergeRefreshedDraft } from '@/api/cache/drafts'
import { t } from '@/i18n'
import { defaultConnectivityConfig, sameConnectivityConfig, validConnectivityConfig } from './config'
import { connectivityError } from './presentation'

export function useConnectivitySettings(snapshot: ShallowRef<ConnectivitySnapshot | null>, refresh: () => Promise<void>) {
  const draft = reactive<ConnectivityConfig>({ ...defaultConnectivityConfig })
  const username = shallowRef('')
  const user = shallowRef<ConnectivityUser | null>(null)
  const baseline = shallowRef<ConnectivityConfig | null>(null)
  const baselineUsername = shallowRef('')
  const busy = shallowRef(false)
  const resolving = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const saved = shallowRef(false)
  let disposed = false
  let resolveGeneration = 0
  let resolveController: AbortController | undefined

  watch(snapshot, next => {
    if (!next) return
    const previous = baseline.value
    const accountClean = username.value === baselineUsername.value && (!previous || draft.proxyUserId === previous.proxyUserId)
    mergeRefreshedDraft(draft, next.config, previous ?? undefined)
    baseline.value = { ...next.config }
    if (accountClean) {
      user.value = next.user
      username.value = next.user?.username ?? ''
      baselineUsername.value = username.value
    }
  })
  const accountResolved = computed(() => username.value.trim()
    ? user.value?.id === draft.proxyUserId && user.value.username === username.value.trim()
    : draft.proxyUserId === 0 || (baselineUsername.value === '' && draft.proxyUserId === baseline.value?.proxyUserId))
  const dirty = computed(() => baseline.value !== null && (!sameConnectivityConfig(draft, baseline.value) || username.value.trim() !== baselineUsername.value))
  const valid = computed(() => validConnectivityConfig(draft) && accountResolved.value)
  const canRun = computed(() => baseline.value !== null && baseline.value.proxyUserId > 0 && !dirty.value && valid.value && !busy.value && !resolving.value)

  async function resolve(): Promise<void> {
    const value = username.value.trim()
    if (!value || value.length > 100 || disposed) return
    resolveController?.abort()
    resolveController = new AbortController()
    const current = ++resolveGeneration
    resolving.value = true
    error.value = null
    try {
      const result = await connectivityApi.resolve(value, resolveController.signal)
      if (disposed || current !== resolveGeneration || username.value.trim() !== value) return
      user.value = result
      username.value = result.username
      draft.proxyUserId = result.id
      saved.value = false
    } catch (caught) {
      if (!disposed && current === resolveGeneration && !resolveController.signal.aborted) error.value = connectivityError(caught, 'hostConnectivity.resolveFailed')
    } finally {
      if (!disposed && current === resolveGeneration) resolving.value = false
    }
  }

  function clearUser(): void {
    resolveGeneration++
    resolveController?.abort()
    resolving.value = false
    username.value = ''
    user.value = null
    draft.proxyUserId = 0
    draft.scheduledEnabled = false
    saved.value = false
  }

  async function save(): Promise<boolean> {
    if (disposed || busy.value) return false
    if (!dirty.value) return true
    if (!valid.value) { error.value = t(accountResolved.value ? 'hostConnectivity.invalidConfig' : 'hostConnectivity.resolveRequired'); return false }
    const submitted = { ...draft }
    const value = { ...submitted, probeUrl: draft.probeUrl.trim() }
    const submittedUsername = username.value.trim()
    busy.value = true
    error.value = null
    saved.value = false
    try {
      await connectivityApi.save(value)
      if (disposed) return false
      mergeRefreshedDraft(draft, value, submitted)
      baseline.value = value
      baselineUsername.value = submittedUsername
      saved.value = true
      await refresh()
      return true
    } catch (caught) {
      if (!disposed) { error.value = connectivityError(caught, 'hostConnectivity.saveFailed'); }
      return false
    } finally {
      if (!disposed) busy.value = false
    }
  }

  onScopeDispose(() => { disposed = true; resolveGeneration++; resolveController?.abort() })
  return { draft, username, user, busy, resolving, error, saved, dirty, valid, canRun, resolve, clearUser, save }
}
