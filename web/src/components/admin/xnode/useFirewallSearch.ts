import { onScopeDispose, shallowRef, watch } from 'vue'
import { firewallApi, type FirewallBan } from '@/api/firewall'
import { adminOperationsApi } from '@/api/adminOperations'
import { useDurableCommand } from '@/composables/useDurableCommand'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

export function useFirewallSearch() {
  const items = shallowRef<FirewallBan[]>([])
  const query = shallowRef(''), cursor = shallowRef(''), nextCursor = shallowRef('')
  const history = shallowRef<string[]>([])
  const loading = shallowRef(true), error = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  let controller: InstanceType<typeof globalThis.AbortController> | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  const release = useDurableCommand({ errorKey: 'firewall.saveFailed', loadReceipt: adminOperationsApi.getOperation, onTerminal: async receipt => { if (receipt.status === 'succeeded') await load() } })
  async function load() {
    controller?.abort(); controller = new globalThis.AbortController()
    const ticket = latest.begin(); loading.value = true; error.value = null
    try {
      const value = await firewallApi.bans(query.value.trim(), cursor.value, controller.signal)
      if (latest.isCurrent(ticket)) { items.value = value.items; nextCursor.value = value.nextCursor ?? '' }
    } catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'firewall.searchFailed') }
    finally { if (latest.isCurrent(ticket)) loading.value = false }
  }
  function next() { if (nextCursor.value) { history.value = [...history.value, cursor.value]; cursor.value = nextCursor.value; void load() } }
  function previous() { if (history.value.length) { const before = [...history.value]; cursor.value = before.pop()!; history.value = before; void load() } }
  async function unban(ban: FirewallBan) {
    const body = { prefix: ban.prefix, generation: ban.generation, hours: 0 }
    await release.execute('firewall-unban', JSON.stringify(body), key => firewallApi.grant(body, key))
  }
  watch(query, () => {
    latest.invalidate(); controller?.abort(); if (timer) clearTimeout(timer)
    cursor.value = ''; history.value = []; nextCursor.value = ''; loading.value = true
    timer = setTimeout(() => void load(), 300)
  })
  onScopeDispose(() => { controller?.abort(); latest.dispose(); if (timer) clearTimeout(timer) })
  void load()
  return { items, query, loading, error, nextCursor, history, release, load, next, previous, unban }
}
