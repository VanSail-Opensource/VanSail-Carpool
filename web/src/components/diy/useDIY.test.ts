import { defineComponent, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import { xnodeMemberApi, type DIYView } from '@/api/xnodeMember'
import { ApiError } from '@/api/http'
import { useDIY } from './useDIY'

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

it('retains both drafts through validation, a lost reply and a revision conflict; reset touches options only', async () => {
  vi.useFakeTimers()
  const route = { id: 'ab', inboundNodeId: 'a', outboundNodeId: 'b', inboundEndpointId: 'a-host', outboundEndpointId: 'b-host', templateId: 'one' }
  const view: DIYView = { settingsRevision: 0, overrides: {}, routing: { plan: { revision: 0, phase: 'active', choices: [route] }, nodes: [], templates: [], paths: [], endpoints: [] }, fields: [{ key: 'tcpFastOpen', kind: 'fast-open', group: 'sockopt', unit: '', minimum: -2147483648, maximum: 2147483647, advanced: true }], compatibility: [] }
  vi.spyOn(xnodeMemberApi, 'diy').mockResolvedValue(view)
  const preview = vi.spyOn(xnodeMemberApi, 'previewDIY').mockResolvedValue({ valid: false, fieldErrors: { routing: 'routing_invalid_rule' }, compatibility: [], paths: [] })
  const save = vi.spyOn(xnodeMemberApi, 'saveDIY').mockRejectedValue(new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }))
  let state!: ReturnType<typeof useDIY>
  const wrapper = mount(defineComponent({ setup() { state = useDIY(); return () => null } }))
  await flushPromises()
  state.overrides.value = { tcpFastOpen: false }; state.routeDraft([{ ...route, templateId: 'two' }]); await nextTick()
  await state.save()
  expect(save).not.toHaveBeenCalled()
  expect(state.choices.value[0]?.templateId).toBe('two')
  preview.mockResolvedValue({ valid: true, fieldErrors: {}, compatibility: [], paths: [] })
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
