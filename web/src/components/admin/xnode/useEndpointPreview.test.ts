import { defineComponent, reactive, nextTick, shallowRef } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { XNodeEndpointPreview } from '@/api/xnode'
import { endpointDraft } from './endpointForm'
const mocks = vi.hoisted(() => ({ previewEndpoint: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: mocks }))
import { useEndpointPreview } from './useEndpointPreview'
let state: ReturnType<typeof useEndpointPreview>
const draft = reactive(endpointDraft()), invalid = shallowRef(false)
const response = (protocol = 'vless'): XNodeEndpointPreview => ({ valid: true, metadata: { protocol, transport: 'xhttp', security: 'tls' }, inbounds: [], fields: [], fieldErrors: [], clients: [] })
const Harness = defineComponent({ setup() { state = useEndpointPreview(() => draft, () => invalid.value); return () => null } })
beforeEach(() => { vi.resetAllMocks(); vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }); Object.assign(draft, endpointDraft()); invalid.value = false })
afterEach(() => vi.useRealTimers())
it('ignores superseded responses even when a transport ignores abort, and gates current saves', async () => {
  let finish!: (value: XNodeEndpointPreview) => void
  mocks.previewEndpoint.mockImplementationOnce(() => new Promise(resolve => { finish = resolve })).mockResolvedValue(response('trojan'))
  const wrapper = mount(Harness)
  await vi.advanceTimersByTimeAsync(300)
  const signal = mocks.previewEndpoint.mock.calls[0]![1] as AbortSignal
  draft.name = 'Changed'; await nextTick()
  expect(signal.aborted).toBe(true)
  expect(state.valid.value).toBe(false)
  finish(response('old')); await flushPromises()
  expect(state.result.value).toBeNull()
  await vi.advanceTimersByTimeAsync(300); await flushPromises()
  expect(state.result.value?.metadata.protocol).toBe('trojan')
  expect(state.valid.value).toBe(true)
  draft.address = 'new.example'
  expect(state.valid.value).toBe(false)
  wrapper.unmount()
})
it('retains drafts on errors, supports retry and aborts observers on disposal', async () => {
  mocks.previewEndpoint.mockRejectedValueOnce(new Error('offline')).mockResolvedValue(response())
  const wrapper = mount(Harness)
  draft.name = 'Retained'; await nextTick(); await vi.advanceTimersByTimeAsync(300); await flushPromises()
  expect(state.error.value).toBeTruthy()
  expect(draft.name).toBe('Retained')
  expect(state.valid.value).toBe(false)
  await state.load()
  expect(state.valid.value).toBe(true)
  invalid.value = true; await nextTick()
  expect(state.valid.value).toBe(false)
  expect(state.pending.value).toBe(false)
  const calls = mocks.previewEndpoint.mock.calls.length
  wrapper.unmount(); await vi.advanceTimersByTimeAsync(2000)
  expect(mocks.previewEndpoint).toHaveBeenCalledTimes(calls)
})
