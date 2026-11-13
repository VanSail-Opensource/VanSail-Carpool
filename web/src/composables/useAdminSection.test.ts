import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { useAdminSection } from './useAdminSection'

function setup() {
  let section!: ReturnType<typeof useAdminSection<{ id: string }>>
  const wrapper = mount(defineComponent({ setup() { section = useAdminSection('combos', { immediate: false }); return () => h('div') } }))
  return { section, wrapper }
}
beforeEach(() => vi.spyOn(api, 'getAdminResource').mockResolvedValue({ items: [{ id: 'confirmed' }] }))
afterEach(() => vi.restoreAllMocks())

describe('administrator mutation lifetime', () => {
  it('admits one mutation at a time and retires pre-mutation reads', async () => {
    let read!: (value: { items: { id: string }[] }) => void
    vi.mocked(api.getAdminResource).mockReturnValueOnce(new Promise(resolve => { read = resolve }))
    const { section, wrapper } = setup()
    const pendingRead = section.load()
    let finish!: () => void
    const action = vi.fn(() => new Promise<void>(resolve => { finish = resolve }))
    const pendingAction = section.perform(action)
    expect(await section.perform(action)).toBe(false)
    read({ items: [{ id: 'before-write' }] })
    await pendingRead
    expect(section.items.value).toEqual([])
    finish()
    expect(await pendingAction).toBe(true)
    expect(action).toHaveBeenCalledOnce()
    expect(section.items.value).toEqual([{ id: 'confirmed' }])
    wrapper.unmount()
  })

  it('does not refresh or present success when a confirmed write finishes after disposal', async () => {
    const { section, wrapper } = setup()
    let finish!: () => void
    const action = vi.fn(() => new Promise<void>(resolve => { finish = resolve }))
    const pending = section.perform(action)
    wrapper.unmount()
    finish()
    expect(await pending).toBe(false)
    await section.load()
    expect(await section.perform(action)).toBe(false)
    expect(api.getAdminResource).not.toHaveBeenCalled()
    expect(action).toHaveBeenCalledOnce()
  })

  it('preserves confirmed mutation success when only its follow-up read fails', async () => {
    const { section, wrapper } = setup()
    vi.mocked(api.getAdminResource).mockRejectedValueOnce(new Error('refresh unavailable'))
    expect(await section.perform(async () => undefined)).toBe(true)
    expect(section.error.value).toBeTruthy()
    expect(section.busy.value).toBe(false)
    wrapper.unmount()
  })
})
