import { mount } from '@vue/test-utils'
import { defineComponent, nextTick, shallowRef } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { connectivityApi, type ConnectivitySnapshot, type ConnectivityUser } from '@/api/connectivity'
import { setLocale } from '@/i18n'
import { defaultConnectivityConfig } from './config'
import { useConnectivitySettings } from './useConnectivitySettings'

vi.mock('@/api/connectivity', () => ({ connectivityApi: { resolve: vi.fn(), save: vi.fn() } }))
const wrappers: ReturnType<typeof mount>[] = []
function snapshot(): ConnectivitySnapshot {
  return { config: { ...defaultConnectivityConfig, proxyUserId: 41 }, user: { id: 41, username: 'connectivity-monitor', status: 'ACTIVE' }, run: null, hosts: [], stale: false, errorCode: '' }
}
function harness() {
  const current = shallowRef<ConnectivitySnapshot | null>(null)
  const refresh = vi.fn().mockResolvedValue(undefined)
  let settings!: ReturnType<typeof useConnectivitySettings>
  const wrapper = mount(defineComponent({ setup() { settings = useConnectivitySettings(current, refresh); return () => null } }))
  wrappers.push(wrapper)
  return { current, refresh, settings }
}
beforeEach(() => { vi.clearAllMocks(); setLocale('en') })
afterEach(() => { for (const wrapper of wrappers.splice(0)) wrapper.unmount() })

describe('connectivity configuration state', () => {
  it('preserves edited fields across polling and writes one atomic configuration', async () => {
    const { current, refresh, settings } = harness()
    current.value = snapshot()
    await nextTick()
    settings.draft.intervalSeconds = 777
    settings.draft.maxRetries = 2
    settings.draft.retryIntervalSeconds = 3
    current.value = { ...snapshot(), config: { ...snapshot().config, intervalSeconds: 600, timeoutSeconds: 20, maxRetries: 5, retryIntervalSeconds: 4 } }
    await nextTick()
    expect(settings.draft.intervalSeconds).toBe(777)
    expect(settings.draft.timeoutSeconds).toBe(20)
    expect(settings.draft.maxRetries).toBe(2)
    expect(settings.draft.retryIntervalSeconds).toBe(3)
    expect(settings.canRun.value).toBe(false)
    vi.mocked(connectivityApi.save).mockResolvedValue(undefined)
    await settings.save()
    expect(connectivityApi.save).toHaveBeenCalledWith({ ...snapshot().config, intervalSeconds: 777, timeoutSeconds: 20, maxRetries: 2, retryIntervalSeconds: 3 })
    expect(refresh).toHaveBeenCalledOnce()
    expect(settings.canRun.value).toBe(true)
  })

  it('requires saving retry edits before running and preserves zero retries', async () => {
    const { current, settings } = harness()
    current.value = snapshot()
    await nextTick()
    settings.draft.maxRetries = 0
    expect(settings.dirty.value).toBe(true)
    expect(settings.valid.value).toBe(true)
    expect(settings.canRun.value).toBe(false)
    vi.mocked(connectivityApi.save).mockResolvedValue(undefined)
    expect(await settings.save()).toBe(true)
    expect(connectivityApi.save).toHaveBeenCalledWith({ ...snapshot().config, maxRetries: 0 })
    expect(settings.canRun.value).toBe(true)
    settings.draft.retryIntervalSeconds = 0
    expect(await settings.save()).toBe(false)
    expect(connectivityApi.save).toHaveBeenCalledOnce()
    expect(settings.canRun.value).toBe(false)
  })

  it('requires the edited exact username to resolve before saving or running', async () => {
    const { current, settings } = harness()
    current.value = snapshot()
    await nextTick()
    settings.username.value = 'other-monitor'
    expect(settings.valid.value).toBe(false)
    expect(settings.canRun.value).toBe(false)
    await settings.save()
    expect(connectivityApi.save).not.toHaveBeenCalled()
    vi.mocked(connectivityApi.resolve).mockResolvedValue({ id: 42, username: 'other-monitor', status: 'ACTIVE' })
    await settings.resolve()
    expect(settings.draft.proxyUserId).toBe(42)
    expect(settings.valid.value).toBe(true)
    expect(settings.canRun.value).toBe(false)
  })

  it('does not restore an account cleared while resolution is in flight', async () => {
    const { current, settings } = harness()
    current.value = snapshot()
    await nextTick()
    let finish!: (user: ConnectivityUser) => void
    vi.mocked(connectivityApi.resolve).mockReturnValue(new Promise(resolve => { finish = resolve }))
    const pending = settings.resolve()
    const signal = vi.mocked(connectivityApi.resolve).mock.calls[0]![1]
    settings.clearUser()
    finish({ id: 41, username: 'connectivity-monitor', status: 'ACTIVE' })
    await pending
    expect(signal?.aborted).toBe(true)
    expect(settings.draft.proxyUserId).toBe(0)
    expect(settings.draft.scheduledEnabled).toBe(false)
    expect(settings.user.value).toBeNull()
  })

  it('permits disabling scheduling and manual retries for an unavailable saved account', async () => {
    const { current, settings } = harness()
    current.value = { ...snapshot(), user: null, stale: true, errorCode: 'CONNECTIVITY_ACCOUNT_UNAVAILABLE', config: { ...snapshot().config, scheduledEnabled: true } }
    await nextTick()
    expect(settings.username.value).toBe('')
    expect(settings.canRun.value).toBe(true)
    settings.draft.scheduledEnabled = false
    expect(settings.valid.value).toBe(true)
    vi.mocked(connectivityApi.save).mockResolvedValue(undefined)
    expect(await settings.save()).toBe(true)
    expect(connectivityApi.save).toHaveBeenCalledWith({ ...snapshot().config, scheduledEnabled: false })
    settings.username.value = 'replacement-monitor'
    expect(settings.valid.value).toBe(false)
    expect(settings.canRun.value).toBe(false)
  })
})
