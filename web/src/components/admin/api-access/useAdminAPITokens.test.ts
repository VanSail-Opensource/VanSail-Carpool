import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { PanelAPITokenIssued } from '@/api/adminAPITokens'
const mock = vi.hoisted(() => ({ list: vi.fn(), create: vi.fn(), revoke: vi.fn() }))
vi.mock('@/api/adminAPITokens', () => ({ adminAPITokensApi: mock }))
import { useAdminAPITokens } from './useAdminAPITokens'

const issued: PanelAPITokenIssued = { token: 'vsc_' + 'A'.repeat(43), item: { id: 'one', name: 'Assistant', prefix: 'vsc_AAAAAAAA', scopes: ['xnode:write'], createdAt: '2026-10-15T00:00:00Z', expiresAt: null, lastUsedAt: null, revokedAt: null } }
function setup() {
  let state!: ReturnType<typeof useAdminAPITokens>
  const wrapper = mount(defineComponent({ setup() { state = useAdminAPITokens(); return () => h('div') } }))
  return { wrapper, state }
}
describe('administrator token secret lifecycle', () => {
  beforeEach(() => { vi.resetAllMocks(); mock.list.mockResolvedValue({ items: [] }); mock.create.mockResolvedValue(issued); mock.revoke.mockResolvedValue(undefined) })
  it('clears the one-time secret on pagehide and revocation', async () => {
    const { wrapper, state } = setup(); await flushPromises()
    await state.create({ name: 'Assistant', scopes: ['xnode:write'], expiresInDays: 30 }); expect(state.issued.value?.token).toBe(issued.token)
    window.dispatchEvent(new Event('pagehide')); expect(state.issued.value).toBeNull()
    await state.create({ name: 'Assistant', scopes: ['xnode:write'], expiresInDays: 30 })
    await state.revoke('one'); expect(state.issued.value).toBeNull(); expect(mock.revoke).toHaveBeenCalledWith('one')
    wrapper.unmount()
  })
  it('does not retain a secret returned after navigation', async () => {
    let resolve!: (value: PanelAPITokenIssued) => void
    mock.create.mockReturnValue(new Promise<PanelAPITokenIssued>(done => { resolve = done }))
    const { wrapper, state } = setup(); await flushPromises()
    const pending = state.create({ name: 'Assistant', scopes: ['xnode:write'], expiresInDays: 30 }); wrapper.unmount(); resolve(issued); await pending
    expect(state.issued.value).toBeNull()
  })
  it('does not reintroduce a pending secret after pagehide', async () => {
    let resolve!: (value: PanelAPITokenIssued) => void
    mock.create.mockReturnValue(new Promise<PanelAPITokenIssued>(done => { resolve = done }))
    const { wrapper, state } = setup(); await flushPromises()
    const pending = state.create({ name: 'Assistant', scopes: ['xnode:write'], expiresInDays: 30 })
    window.dispatchEvent(new Event('pagehide')); resolve(issued); await pending
    expect(state.issued.value).toBeNull(); expect(state.items.value[0]?.id).toBe('one'); wrapper.unmount()
  })
  it('does not overwrite a newly issued record with an earlier list response', async () => {
    let resolve!: (value: { items: [] }) => void
    mock.list.mockReturnValueOnce(new Promise<{ items: [] }>(done => { resolve = done }))
    const { wrapper, state } = setup()
    await state.create({ name: 'Assistant', scopes: ['xnode:write'], expiresInDays: 30 }); resolve({ items: [] }); await flushPromises()
    expect(state.items.value[0]?.id).toBe('one'); wrapper.unmount()
  })
  it('cannot restore a deleted token from a stale list response', async () => {
    let resolve!: (value: { items: typeof issued.item[] }) => void
    mock.list.mockReturnValueOnce(new Promise(done => { resolve = done })).mockResolvedValue({ items: [] })
    const { wrapper, state } = setup()
    state.items.value = [issued.item]
    await state.revoke('one')
    resolve({ items: [issued.item] }); await flushPromises()
    expect(state.items.value).toEqual([])
    wrapper.unmount()
  })

})
