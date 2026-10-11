import { defineComponent, reactive, shallowRef, nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { IPLookupLiveDetails } from '@/api/ipLookup'

const mocks = vi.hoisted(() => ({ details: vi.fn() }))
vi.mock('@/api/ipLookup', () => ({ ipLookupApi: mocks }))
const account = reactive({ user: { id: 'member' } })
vi.mock('@/stores/session', () => ({ useSessionStore: () => account }))
import { useLiveIPDetails } from './useLiveIPDetails'

const ip = shallowRef('')
const key = shallowRef('')
let state: ReturnType<typeof useLiveIPDetails>
const Harness = defineComponent({ setup() { state = useLiveIPDetails(() => ip.value, () => key.value); return () => null } })

describe('live IP details ownership and request boundaries', () => {
  beforeEach(() => { vi.resetAllMocks(); account.user.id = 'member' })
  it('queries the requested address once per report identity and refreshes without submission', async () => {
    ip.value = '8.8.8.9'
    key.value = 'cached-report:date'
    mocks.details.mockImplementation(async (address: string) => ({ ip: address }))
    const wrapper = mount(Harness)
    await flushPromises()
    expect(mocks.details).toHaveBeenCalledTimes(1)
    expect(mocks.details.mock.calls[0]![0]).toBe('8.8.8.9')
    key.value = 'cached-report:date'
    await nextTick()
    expect(mocks.details).toHaveBeenCalledTimes(1)
    await state.refresh()
    expect(mocks.details).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
  it('aborts old address/account requests and rejects late results', async () => {
    ip.value = '1.1.1.1'
    key.value = 'report'
    const resolvers: Array<(value: IPLookupLiveDetails) => void> = []
    mocks.details.mockImplementation(() => new Promise(resolve => resolvers.push(resolve)))
    const wrapper = mount(Harness)
    const firstSignal = mocks.details.mock.calls[0]![1] as AbortSignal
    ip.value = '8.8.8.8'
    await nextTick()
    expect(firstSignal.aborted).toBe(true)
    resolvers[0]!({ ip: '1.1.1.1' } as IPLookupLiveDetails)
    await flushPromises()
    expect(state.details.value).toBeNull()
    const secondSignal = mocks.details.mock.calls[1]![1] as AbortSignal
    account.user.id = 'replacement'
    await nextTick()
    expect(secondSignal.aborted).toBe(true)
    wrapper.unmount()
    expect((mocks.details.mock.calls[2]![1] as AbortSignal).aborted).toBe(true)
  })
  it('publishes partial sections and retains them when an explicit refresh fails', async () => {
    ip.value = '1.1.1.1'; key.value = 'report'
    mocks.details.mockImplementation(async (address, _signal, refresh, emit) => {
      if (refresh) throw new Error('source unavailable')
      emit({ type: 'section', ip: address, section: 'ptr', data: { domains: ['one.one.one.one'] } })
      emit({ type: 'progress', ip: address, progress: { phase: 'asns', completed: 3, total: 8 } })
      expect(state.details.value?.ptr?.domains).toEqual(['one.one.one.one'])
      return { ip: address, ptr: { domains: ['one.one.one.one'] } }
    })
    const wrapper = mount(Harness)
    await flushPromises()
    expect(mocks.details.mock.calls[0]![2]).toBe(false)
    await state.refresh()
    expect(mocks.details.mock.calls[1]![2]).toBe(true)
    expect(state.details.value?.ptr?.domains).toEqual(['one.one.one.one'])
    expect(state.errorCode.value).toBe('IP_DETAILS_UNAVAILABLE')
    wrapper.unmount()
  })
})
