import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { hourlyApi } from '@/api/hourly'
import type { OperationReceipt } from '@/api/types'
import { hourlyFixture } from '../../../fixtures/hourly-data'
import { useHourly } from './useHourly'

const track = vi.hoisted(() => vi.fn())
vi.mock('@/composables/useOperationReceipt', async () => {
  const { shallowRef } = await import('vue')
  return { useOperationReceipt: () => ({ polling: shallowRef(false), terminal: shallowRef(false), track }) }
})
afterEach(() => { vi.restoreAllMocks(); track.mockClear() })

describe('hourly action disposal', () => {
  it('does not restart receipt tracking or refresh after a late activation response', async () => {
    const view = hourlyFixture(new URLSearchParams()).view
    const get = vi.spyOn(hourlyApi, 'get').mockResolvedValue(view)
    let accept!: (receipt: OperationReceipt) => void
    vi.spyOn(hourlyApi, 'activate').mockReturnValue(new Promise(resolve => { accept = resolve }))
    let state!: ReturnType<typeof useHourly>
    const wrapper = mount(defineComponent({ setup() { state = useHourly(); return () => null } }))
    await flushPromises()
    const pending = state.activate(view.squads[0]!)
    wrapper.unmount()
    accept({ id: 'accepted', kind: 'hourly_access', status: 'queued', createdAt: '', updatedAt: '', completedAt: null })
    expect(await pending).toBe(false)
    expect(track).not.toHaveBeenCalled()
    expect(get).toHaveBeenCalledTimes(1)
  })
})
