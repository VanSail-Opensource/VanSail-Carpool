import { defineComponent, nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { AutomaticLeasePage } from '@/api/firewall'

const mocks = vi.hoisted(() => ({ leases: vi.fn() }))
vi.mock('@/api/firewall', () => ({ firewallApi: mocks }))
import { useAutomaticLeases } from './useAutomaticLeases'

let state: ReturnType<typeof useAutomaticLeases>
const Harness = defineComponent({ setup() { state = useAutomaticLeases(); return () => null } })
const page = (total = 75): AutomaticLeasePage => ({ items: [], total, page: 1, pageSize: 25, statistics: { active: 75, ipv4: 50, ipv6: 25, created: '9007199254740993', renewed: '0', expired: '0', rejected: '0' }, settings: { enabled: true, durationHours: 24 }, fetchedAt: new Date().toISOString() })

beforeEach(() => {
  vi.resetAllMocks()
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
})
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

it('rejects a late search response and preserves a marked snapshot after a failed refresh', async () => {
  let finish!: (value: AutomaticLeasePage) => void
  mocks.leases.mockImplementationOnce(() => new Promise(resolve => { finish = resolve })).mockResolvedValue(page())
  const wrapper = mount(Harness)
  const signal = mocks.leases.mock.calls[0]![2] as AbortSignal
  state.query.value = '8.8'
  await nextTick()
  expect(signal.aborted).toBe(true)
  finish(page())
  await flushPromises()
  expect(state.data.value).toBeNull()
  await vi.advanceTimersByTimeAsync(300)
  await flushPromises()
  expect(mocks.leases.mock.calls.at(-1)?.slice(0, 2)).toEqual(['8.8', 1])
  expect(state.data.value?.statistics.created).toBe('9007199254740993')
  mocks.leases.mockRejectedValue(new Error('offline'))
  await state.load()
  expect(state.error.value).toBeTruthy()
  expect(state.data.value?.total).toBe(75)
  const calls = mocks.leases.mock.calls.length
  wrapper.unmount()
  await vi.advanceTimersByTimeAsync(20000)
  expect(mocks.leases).toHaveBeenCalledTimes(calls)
})

it('pauses hidden polling and clears volatile rows and outstanding work on session end', async () => {
  mocks.leases.mockResolvedValue(page())
  const wrapper = mount(Harness)
  await flushPromises()
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
  document.dispatchEvent(new Event('visibilitychange'))
  await vi.advanceTimersByTimeAsync(20000)
  expect(mocks.leases).toHaveBeenCalledTimes(1)
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  document.dispatchEvent(new Event('visibilitychange'))
  await flushPromises()
  expect(mocks.leases).toHaveBeenCalledTimes(2)
  window.dispatchEvent(new Event('txc:session-cleared'))
  expect(state.data.value).toBeNull()
  expect((mocks.leases.mock.calls[1]![2] as AbortSignal).aborted).toBe(true)
  await vi.advanceTimersByTimeAsync(20000)
  expect(mocks.leases).toHaveBeenCalledTimes(2)
  wrapper.unmount()
})

it('returns to an existing page when expiry shrinks the result set', async () => {
  mocks.leases.mockResolvedValue(page())
  const wrapper = mount(Harness)
  await flushPromises()
  state.page.value = 3
  await nextTick()
  mocks.leases.mockResolvedValue(page(10))
  await vi.advanceTimersByTimeAsync(300)
  await flushPromises()
  expect(state.page.value).toBe(1)
  await vi.advanceTimersByTimeAsync(300)
  await flushPromises()
  expect(state.data.value?.total).toBe(10)
  wrapper.unmount()
})

it('preserves pager metadata while moving pages and resets search to one', async () => {
 mocks.leases.mockResolvedValue(page())
 const wrapper=mount(Harness);await flushPromises()
 state.page.value=2;await nextTick()
 expect(state.loading.value).toBe(true)
 expect(state.data.value?.total).toBe(75)
 state.query.value='8.8';await nextTick();expect(state.page.value).toBe(1)
 wrapper.unmount()
})
