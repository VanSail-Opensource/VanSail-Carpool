import { api } from '@/api/client'
import { subscriptionApi } from '@/api/subscription'
import { xnodeMemberApi } from '@/api/xnodeMember'
import { paygApi } from '@/api/payg'
import { hourlyApi } from '@/api/hourly'
import { preferencesApi } from '@/api/preferences'
import { featuresApi } from '@/api/features'
import { memberOperationsApi } from '@/api/memberOperations'
import { ipLookupApi } from '@/api/ipLookup'
import { displayCurrencyApi } from '@/api/displayCurrency'

// Reuse the actual shell and constructed Home responses without real accounts.
const url = new URL(location.href)
url.searchParams.set('shell', '')
if (!url.searchParams.has('mode')) url.searchParams.set('mode', 'knowledge')
if (url.searchParams.has('new-user')) url.searchParams.set('zero-balance', '')
history.replaceState(null, '', url)
const state = { calls: [] as string[], unexpected: [] as string[], pending: 0, instrumented: false }
Object.assign(window, { __knowledgebaseAudit: state })
const nativeFetch = window.fetch.bind(window)
window.fetch = async (input, init) => {
  const request = new Request(input, init)
  const target = new URL(request.url)
  if (target.origin !== location.origin || target.pathname.startsWith('/api/')) {
    state.unexpected.push(`${request.method} ${target.pathname}`)
    throw new Error('Unexpected Knowledgebase fixture API request')
  }
  return nativeFetch(input, init)
}
// Install after the fixture mocks exist but before the shell can start reads.
Object.assign(window, { __subscriptionAuditSetup: () => {
  for (const [clientName, client] of Object.entries({ api, subscriptionApi, xnodeMemberApi, paygApi, hourlyApi, preferencesApi, featuresApi, memberOperationsApi, ipLookupApi, displayCurrencyApi })) {
    for (const [name, method] of Object.entries(client)) {
      if (typeof method !== 'function') continue
      Object.assign(client, { [name]: (...args: unknown[]) => {
        state.calls.push(`${clientName}.${name}`)
        state.pending++
        try {
          return Promise.resolve(Reflect.apply(method, client, args)).finally(() => { state.pending-- })
        } catch (error) { state.pending--; throw error }
      } })
    }
  }
  state.instrumented = true
} })
await import('./subscription-audit')
