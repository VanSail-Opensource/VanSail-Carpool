import { defineComponent, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { GroupMemberTag } from '@/api/preferences'

const mocks = vi.hoisted(() => ({ getTag: vi.fn(), updateTag: vi.fn(), restoreRef: vi.fn() }))
vi.mock('@/api/preferences', () => ({ preferencesApi: mocks }))
vi.mock('@/api/cache/restore', () => ({ restoreRef: mocks.restoreRef }))
const account = reactive<{ user: { id: string } | null }>({ user: { id: 'first' } })
vi.mock('@/stores/session', () => ({ useSessionStore: () => account }))
import { useSettings } from './useSettings'

let state: ReturnType<typeof useSettings>
const Harness = defineComponent({ setup() { state = useSettings(); return () => null } })
const tag = (value: string): GroupMemberTag => ({ groupJoined: true, editable: true, tag: value, reasonCode: '' })
beforeEach(() => { vi.resetAllMocks(); account.user = { id: 'first' }; mocks.getTag.mockResolvedValue(tag('First')) })

describe('settings tag owner and mutation recovery', () => {
  it('shows cached tags but blocks editing until a fresh eligibility read succeeds', async () => {
    mocks.restoreRef.mockImplementationOnce((_path, target) => { target.value = tag('Cached'); return true })
    let resolve!: (value: GroupMemberTag) => void
    mocks.getTag.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const wrapper = mount(Harness)
    expect(state.tagLoading.value).toBe(false)
    expect(state.tag.value?.tag).toBe('Cached')
    expect(state.tagBlocked.value).toBe(true)
    await state.saveTag('Too soon')
    expect(mocks.updateTag).not.toHaveBeenCalled()
    resolve(tag('Fresh'))
    await flushPromises()
    expect(state.tag.value?.tag).toBe('Fresh')
    expect(state.tagBlocked.value).toBe(false)
    wrapper.unmount()
  })

  it('clears the tag immediately and ignores prior-owner reads', async () => {
    let resolve!: (value: GroupMemberTag) => void
    mocks.getTag.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const wrapper = mount(Harness)
    mocks.getTag.mockResolvedValue(tag('Second'))
    account.user = { id: 'second' }
    expect(state.tag.value).toBeNull()
    await flushPromises()
    expect(state.tag.value?.tag).toBe('Second')
    resolve(tag('Retired owner'))
    await flushPromises()
    expect(state.tag.value?.tag).toBe('Second')
    account.user = null
    expect(state.tag.value).toBeNull()
    await state.saveTag('ignored')
    expect(mocks.updateTag).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('keeps a pending write authoritative over a manual read and recovers from failure', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    let reject!: (error: Error) => void
    mocks.updateTag.mockReturnValueOnce(new Promise((_done, fail) => { reject = fail }))
    const saving = state.saveTag('Updated')
    await state.loadTag()
    expect(mocks.getTag).toHaveBeenCalledOnce()
    reject(new Error('constructed refusal'))
    await saving
    expect(state.tag.value?.tag).toBe('First')
    expect(state.tagSaving.value).toBe(false)
    expect(state.tagError.value).toBeTruthy()
    mocks.updateTag.mockResolvedValue(tag('Updated'))
    await state.saveTag('Updated')
    expect(state.tag.value?.tag).toBe('Updated')
    expect(state.tagError.value).toBeNull()
    wrapper.unmount()
  })
  it('rejects late writes after disposal without restarting requests', async () => {
    const wrapper = mount(Harness)
    await flushPromises()
    let resolve!: (value: GroupMemberTag) => void
    mocks.updateTag.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const saving = state.saveTag('Retired')
    wrapper.unmount()
    resolve(tag('Retired'))
    await saving
    await state.loadTag()
    await state.saveTag('Ignored')
    expect(state.tag.value).toBeNull()
    expect(mocks.getTag).toHaveBeenCalledOnce()
    expect(mocks.updateTag).toHaveBeenCalledOnce()
  })
})
