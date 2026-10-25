import { defineComponent, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import { xnodeMemberApi, type DIYView } from '@/api/xnodeMember'
import { ApiError } from '@/api/http'
import { useDIY } from './useDIY'

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

function diyHarness(receive: (state: ReturnType<typeof useDIY>) => void) {
  return defineComponent({ setup() { receive(useDIY()); return () => null } })
}

it('refreshes queued PAYG inventory, preserves drafts and stops polling after disposal', async () => {
  vi.useFakeTimers()
  const billing: DIYView['billing'] = { available: true, changed: false, useQuota: false, remaining: 0, charge: { currency: 'TXB', minor: '0', display: '0.00' }, token: 'synthetic', expiresAt: 9999999999 }
  const view: DIYView = { inventoryPending: true, billing, splitHosts: [], splitProfiles: [], settingsRevision: 0, overrides: {}, routing: { plan: { revision: 0, phase: 'active', choices: [] }, nodes: [], templates: [], paths: [], endpoints: [] }, fields: [], compatibility: [] }
  const reader = vi.spyOn(xnodeMemberApi, 'diy').mockResolvedValueOnce(view).mockResolvedValue({ ...view, inventoryPending: false, routing: { ...view.routing, nodes: [{ id: 'payg-node', name: 'PAYG node' }], endpoints: [{ id: 'payg-host', nodeId: 'payg-node', name: 'PAYG host', address: 'proxy.example', port: 443 }] } })
  vi.spyOn(xnodeMemberApi, 'previewDIY').mockResolvedValue({ billing, valid: true, fieldErrors: {}, compatibility: [], paths: [] })
  let state!: ReturnType<typeof useDIY>
  const component = diyHarness(value => { state = value })
  const wrapper = mount(component)
  await flushPromises()
  state.overrides.value = { tcpFastOpen: false }
  await vi.advanceTimersByTimeAsync(1500)
  await flushPromises()
  expect(reader).toHaveBeenCalledTimes(2)
  expect(state.view.value?.routing.endpoints?.[0]?.id).toBe('payg-host')
  expect(state.overrides.value.tcpFastOpen).toBe(false)
  await vi.advanceTimersByTimeAsync(1500)
  expect(reader).toHaveBeenCalledTimes(2)
  wrapper.unmount()
  reader.mockResolvedValue(view)
  const pending = mount(component)
  await flushPromises()
  const calls = reader.mock.calls.length
  pending.unmount()
  await vi.advanceTimersByTimeAsync(1500)
  expect(reader).toHaveBeenCalledTimes(calls)
})

it('retains both drafts through validation, a lost reply and a revision conflict; reset touches options only', async () => {
  vi.useFakeTimers()
  const route = { id: 'ab', inboundNodeId: 'a', outboundNodeId: 'b', inboundEndpointId: 'a-host', outboundEndpointId: 'b-host', templateId: 'one' }
  const billing: DIYView['billing'] = { available: true, changed: true, useQuota: true, remaining: 5, charge: { currency: 'TXB', minor: '0', display: '0.00' }, token: 'synthetic', expiresAt: 9999999999 }
  const view: DIYView = { billing, splitHosts: [], splitProfiles: [], settingsRevision: 0, overrides: {}, routing: { plan: { revision: 0, phase: 'active', choices: [route] }, nodes: [], templates: [], paths: [], endpoints: [] }, fields: [{ key: 'tcpFastOpen', kind: 'fast-open', group: 'sockopt', unit: '', minimum: -2147483648, maximum: 2147483647, advanced: true }], compatibility: [] }
  vi.spyOn(xnodeMemberApi, 'diy').mockResolvedValue(view)
  const preview = vi.spyOn(xnodeMemberApi, 'previewDIY').mockResolvedValue({ billing, valid: false, fieldErrors: { routing: 'routing_invalid_rule' }, compatibility: [], paths: [] })
  const save = vi.spyOn(xnodeMemberApi, 'saveDIY').mockRejectedValue(new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }))
  let state!: ReturnType<typeof useDIY>
  const wrapper = mount(diyHarness(value => { state = value }))
  await flushPromises()
  state.overrides.value = { tcpFastOpen: false }; state.routeDraft([{ ...route, templateId: 'two' }]); await nextTick()
  await state.save()
  expect(save).not.toHaveBeenCalled()
  expect(state.choices.value[0]?.templateId).toBe('two')
  preview.mockResolvedValue({ billing, valid: true, fieldErrors: {}, compatibility: [], paths: [] })
  await state.save(); await state.save()
  expect(save).toHaveBeenCalledTimes(2)
  expect(save.mock.calls[0]?.[1]).toBe(save.mock.calls[1]?.[1])
  expect(state.overrides.value.tcpFastOpen).toBe(false)
  save.mockRejectedValue(new ApiError(409, { code: 'XNODE_CONFLICT', message: '' }))
  await state.save(); await state.load(true)
  expect(state.choices.value[0]?.templateId).toBe('two')
  expect(state.overrides.value.tcpFastOpen).toBe(false)
  state.reset()
  expect(state.overrides.value).toEqual({})
  expect(state.choices.value[0]?.templateId).toBe('two')
  wrapper.unmount()
})
