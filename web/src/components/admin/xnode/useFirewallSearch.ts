import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { firewallApi, type FirewallFilter, type FirewallRecord } from '@/api/firewall'
import { adminOperationsApi } from '@/api/adminOperations'
import { useDurableCommand } from '@/composables/useDurableCommand'
import { localizedError, useI18n } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

export function useFirewallSearch() {
  const { t } = useI18n()
  const items = shallowRef<FirewallRecord[]>([])
  const query = shallowRef(''), cursor = shallowRef(''), nextCursor = shallowRef('')
  const draft = shallowRef<FirewallFilter>({ op: 'and', children: [] })
  const applied = shallowRef<FirewallFilter>({ op: 'and', children: [] })
  const filterValid = computed(() => validFilter(draft.value) && conditionCount(draft.value) + Number(Boolean(query.value.trim())) <= 32)
  const history = shallowRef<string[]>([])
  const loading = shallowRef(true), error = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  let controller: InstanceType<typeof globalThis.AbortController> | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  const release = useDurableCommand({ errorKey: 'firewall.saveFailed', loadReceipt: adminOperationsApi.getOperation, onTerminal: async receipt => { if (receipt.status === 'succeeded') await load() } })
  async function load() {
    controller?.abort(); controller = new globalThis.AbortController()
    const ticket = latest.begin(); loading.value = true; error.value = null
    if (conditionCount(applied.value) + Number(Boolean(query.value.trim())) > 32) {
      error.value = t('firewall.filterLimit'); loading.value = false
      return
    }
    try {
      const conditions: FirewallFilter[] = query.value.trim() ? [{ field: 'ip', value: query.value.trim() }] : []
      conditions.push(applied.value)
      const value = await firewallApi.records({ filter: { op: 'and', children: conditions }, cursor: cursor.value, limit: 50 }, controller.signal)
      if (latest.isCurrent(ticket)) { items.value = value.items; nextCursor.value = value.nextCursor ?? '' }
    } catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'firewall.searchFailed') }
    finally { if (latest.isCurrent(ticket)) loading.value = false }
  }
  function next() { if (nextCursor.value) { history.value = [...history.value, cursor.value]; cursor.value = nextCursor.value; void load() } }
  function previous() { if (history.value.length) { const before = [...history.value]; cursor.value = before.pop()!; history.value = before; void load() } }
  async function unban(ban: FirewallRecord) {
    if (!ban.banned || !ban.generation) return
    const body = { prefix: ban.prefix, generation: ban.generation, hours: 0 }
    await release.execute('firewall-unban', JSON.stringify(body), key => firewallApi.grant(body, key))
  }
  function applyFilters() {
    if (!filterValid.value) return
    applied.value = structuredClone(draft.value)
    cursor.value = ''; history.value = []; nextCursor.value = ''
    void load()
  }
  watch(query, () => {
    latest.invalidate(); controller?.abort(); if (timer) clearTimeout(timer)
    cursor.value = ''; history.value = []; nextCursor.value = ''; loading.value = true
    timer = setTimeout(() => void load(), 300)
  })
  onScopeDispose(() => { controller?.abort(); latest.dispose(); if (timer) clearTimeout(timer) })
  void load()
  return { items, query, draft, filterValid, applyFilters, loading, error, nextCursor, history, release, load, next, previous, unban }
}

function conditionCount(filter: FirewallFilter): number {
  return filter.op ? (filter.children ?? []).reduce((sum, child) => sum + conditionCount(child), 0) : 1
}

function validFilter(filter: FirewallFilter): boolean {
  if (filter.op) return (filter.children ?? []).every(validFilter)
  if (filter.field === 'detectedAt') {
    if (!filter.from && !filter.to) return false
    return !filter.from || !filter.to || Date.parse(filter.from) <= Date.parse(filter.to)
  }
  if (filter.field === 'hits') return /^\d{1,20}$/.test(filter.value ?? '') && BigInt(filter.value ?? '0') <= 18446744073709551615n
  return Boolean(filter.field && filter.value?.trim())
}
