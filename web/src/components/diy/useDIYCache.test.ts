import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import * as cache from '@/api/cache/restore'
import { xnodeMemberApi, type DIYView } from '@/api/xnodeMember'
import { useDIY } from './useDIY'

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

function snapshot(revision = 1): DIYView {
  return {
    settingsRevision: revision, overrides: { fingerprint: revision === 1 ? 'chrome' : 'firefox' },
    splitHosts: [], splitProfiles: [], compatibility: [],
    fields: [{ key: 'fingerprint', kind: 'enum', group: 'tls', unit: '', minimum: 0, maximum: 0, enum: ['chrome', 'firefox', 'safari'], advanced: false }],
    routing: { plan: { revision, phase: 'active', choices: [] }, nodes: [], endpoints: [], templates: [], paths: [] },
  }
}
function restore(value: DIYView) {
  vi.spyOn(cache, 'restoreCached').mockImplementation((_path, apply) => { apply(structuredClone(value) as never); return true })
}
function harness() {
  let state!: ReturnType<typeof useDIY>
  const wrapper = mount(defineComponent({ setup() { state = useDIY(); return () => null } }))
  return { state, wrapper }
}

it('renders the cached draft immediately and preserves edits made before fresh metadata arrives', async () => {
  vi.useFakeTimers()
  restore(snapshot())
  let finish!: (value: DIYView) => void
  vi.spyOn(xnodeMemberApi, 'diy').mockImplementation(() => new Promise(resolve => { finish = resolve }))
  const preview = vi.spyOn(xnodeMemberApi, 'previewDIY').mockResolvedValue({ valid: true, fieldErrors: {}, compatibility: [], paths: [] })
  const save = vi.spyOn(xnodeMemberApi, 'saveDIY')
  const { state, wrapper } = harness()
  expect(state.view.value?.settingsRevision).toBe(1)
  state.overrides.value = { fingerprint: 'safari' }
  await state.save()
  await vi.advanceTimersByTimeAsync(600)
  expect(preview).not.toHaveBeenCalled()
  expect(save).not.toHaveBeenCalled()
  const fresh = snapshot(2)
  fresh.splitProfiles = [{ id: 'fresh-split', uploadEndpointId: 'a', downloadEndpointId: 'b', mode: 'auto' }]
  finish(fresh)
  await flushPromises()
  expect(state.confirmed.value).toBe(true)
  expect(state.overrides.value.fingerprint).toBe('safari')
  expect(state.splitProfiles.value[0]?.id).toBe('fresh-split')
  expect(state.view.value?.settingsRevision).toBe(2)
  wrapper.unmount()
})

it('adopts fresher settings when the restored draft was untouched', async () => {
  restore(snapshot())
  vi.spyOn(xnodeMemberApi, 'diy').mockResolvedValue(snapshot(2))
  vi.spyOn(xnodeMemberApi, 'previewDIY').mockResolvedValue({ valid: true, fieldErrors: {}, compatibility: [], paths: [] })
  const { state, wrapper } = harness()
  await flushPromises()
  expect(state.overrides.value.fingerprint).toBe('firefox')
  expect(state.choices.value).toEqual(snapshot(2).routing.plan.choices)
  wrapper.unmount()
})

it('keeps cached settings visible after a failed live read without permitting save or cached inventory polling', async () => {
  vi.useFakeTimers()
  restore({ ...snapshot(), inventoryPending: true })
  const read = vi.spyOn(xnodeMemberApi, 'diy').mockRejectedValueOnce(new Error('offline')).mockResolvedValue(snapshot(2))
  const save = vi.spyOn(xnodeMemberApi, 'saveDIY')
  const { state, wrapper } = harness()
  await flushPromises()
  expect(state.view.value?.settingsRevision).toBe(1)
  expect(state.loading.value).toBe(false)
  expect(state.confirmed.value).toBe(false)
  await state.save()
  await vi.advanceTimersByTimeAsync(1600)
  expect(save).not.toHaveBeenCalled()
  expect(read).toHaveBeenCalledTimes(1)
  await state.load()
  expect(state.confirmed.value).toBe(true)
  expect(state.overrides.value.fingerprint).toBe('firefox')
  wrapper.unmount()
})
